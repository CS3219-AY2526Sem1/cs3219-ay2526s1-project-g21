package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Jeffail/leaps/lib/text"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"collab/internal/exec"
	"collab/internal/format"
	"collab/internal/models"
	"collab/internal/room_management"
	"collab/internal/session"
	"collab/internal/utils"
)

type Handlers struct {
	log         *utils.Logger
	runner      runner
	hub         *session.Hub
	roomManager roomManager
}

type runner interface {
	LangSpecPublic(models.Language) (models.LanguageSpec, string, string, [][]string, error)
	RunOnce(ctx context.Context, lang models.Language, code string, limits exec.SandboxLimits) (exec.RunOutput, error)
	RunStream(ctx context.Context, lang models.Language, code string, limits exec.SandboxLimits) ([]models.WSFrame, error)
}

type roomManager interface {
	ValidateRoomAccess(token string) (*models.RoomInfo, error)
	GetRoomStatus(matchId string) (*models.RoomInfo, error)
	RerollQuestion(matchId string) (*models.RoomInfo, error)
	GetActiveRoomForUser(userId string) (*models.RoomInfo, error)
	PublishSessionEnded(event models.SessionEndedEvent) error
	MarkRoomAsEnded(matchID string) error
}

func NewHandlers(log *utils.Logger, roomManager *room_management.RoomManager) *Handlers {
	return NewHandlersWithDeps(log, exec.NewRunner(), session.NewHub(), roomManager)
}

func NewHandlersWithDeps(log *utils.Logger, runner runner, hub *session.Hub, roomManager roomManager) *Handlers {
	return &Handlers{
		log:         log,
		runner:      runner,
		hub:         hub,
		roomManager: roomManager,
	}
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok"))
}

// Get room status by match ID (requires token)
func (h *Handlers) GetRoomStatus(w http.ResponseWriter, r *http.Request) {
	matchId := chi.URLParam(r, "matchId")
	if matchId == "" {
		http.Error(w, "matchId is required", http.StatusBadRequest)
		return
	}

	// Extract token from Authorization header
	authHeader := r.Header.Get("Authorization")
	token, err := utils.ExtractTokenFromHeader(authHeader)
	if err != nil {
		http.Error(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	// Validate token and get room info
	roomInfo, err := h.roomManager.ValidateRoomAccess(token)
	if err != nil {
		http.Error(w, "Unauthorized access", http.StatusUnauthorized)
		return
	}

	writeJSON(w, roomInfo)
}

// GetActiveRoom checks if a user has an active room
func (h *Handlers) GetActiveRoom(w http.ResponseWriter, r *http.Request) {
	userId := chi.URLParam(r, "userId")
	if userId == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	activeRoom, err := h.roomManager.GetActiveRoomForUser(userId)
	if err != nil {
		// No active room found
		writeJSON(w, map[string]interface{}{"active": false})
		return
	}

	// Determine which token to return based on user ID
	var userToken string
	if activeRoom.User1 == userId {
		userToken = activeRoom.Token1
	} else if activeRoom.User2 == userId {
		userToken = activeRoom.Token2
	}

	writeJSON(w, map[string]interface{}{
		"active":  true,
		"matchId": activeRoom.MatchId,
		"status":  activeRoom.Status,
		"token":   userToken,
	})
}

func (h *Handlers) RerollQuestion(w http.ResponseWriter, r *http.Request) {
	matchId := chi.URLParam(r, "matchId")
	if matchId == "" {
		http.Error(w, "matchId is required", http.StatusBadRequest)
		return
	}

	authHeader := r.Header.Get("Authorization")
	token, err := utils.ExtractTokenFromHeader(authHeader)
	if err != nil {
		http.Error(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	roomInfo, err := h.roomManager.ValidateRoomAccess(token)
	if err != nil {
		http.Error(w, "Unauthorized access", http.StatusUnauthorized)
		return
	}

	if roomInfo.MatchId != matchId {
		http.Error(w, "Invalid room", http.StatusBadRequest)
		return
	}

	updated, err := h.roomManager.RerollQuestion(matchId)
	if err != nil {
		switch {
		case errors.Is(err, room_management.ErrNoRerolls):
			http.Error(w, "No rerolls remaining for this room", http.StatusBadRequest)
		case errors.Is(err, room_management.ErrNoAlternativeQuestion):
			http.Error(w, "No different question available right now", http.StatusConflict)
		default:
			http.Error(w, "Failed to reroll question", http.StatusInternalServerError)
			h.log.Error("failed to reroll question", "matchId", matchId, "error", err.Error())
		}
		return
	}

	writeJSON(w, updated)

	if room, ok := h.hub.Get(matchId); ok {
		room.BroadcastAll(models.WSFrame{
			Type: "question",
			Data: models.QuestionUpdate{
				Question:         updated.Question,
				RerollsRemaining: updated.RerollsRemaining,
			},
		})
	}
}

func (h *Handlers) ListLanguages(w http.ResponseWriter, _ *http.Request) {
	languages := []models.Language{models.LangPython, models.LangJava, models.LangCPP}
	resp := make([]models.LanguageSpec, 0, len(languages))
	for _, lang := range languages {
		spec, _, _, _, err := h.runner.LangSpecPublic(lang)
		if err != nil {
			continue
		}
		resp = append(resp, spec)
	}
	writeJSON(w, resp)
}

func (h *Handlers) FormatCode(w http.ResponseWriter, r *http.Request) {
	var req models.FormatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	out, err := format.Format(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, models.FormatResponse{Formatted: out})
}

func (h *Handlers) RunOnce(w http.ResponseWriter, r *http.Request) {
	var req models.RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	limits := exec.SandboxLimits{
		WallTime: 10 * time.Second,
		MemoryB:  512 * 1024 * 1024,
		NanoCPUs: 1_000_000_000,
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()

	out, err := h.runner.RunOnce(ctx, req.Language, req.Code, limits)
	if err != nil {
		switch {
		case errors.Is(err, exec.ErrDockerUnavailable):
			http.Error(w, "sandbox_unavailable", http.StatusServiceUnavailable)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	writeJSON(w, models.RunResult{
		Stdout: out.Stdout, Stderr: out.Stderr, Exit: out.Exit, TimedOut: out.TimedOut,
	})
}

/*** Collab WebSocket: shared editor + run streaming (no question fetching here) ***/
var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (h *Handlers) CollabWS(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")

	// Extract token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token required", http.StatusUnauthorized)
		return
	}

	// Validate token and get room info
	roomInfo, err := h.roomManager.ValidateRoomAccess(token)
	if err != nil {
		http.Error(w, "Unauthorized access", http.StatusUnauthorized)
		return
	}

	// Verify the session ID matches the room
	if roomInfo.MatchId != sessionID {
		http.Error(w, "Invalid room", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	client := session.NewClient(conn)
	room := h.hub.GetOrCreate(sessionID)
	if room.GetClientCount() >= 2 {
		_ = conn.WriteJSON(models.WSFrame{
			Type: "error",
			Data: "room_full",
		})
		return
	}

	// Set up session end handler for this room (only once)
	if room.GetClientCount() == 0 {
		room.SetSessionEndHandler(func(sessID string, finalCode string, lang models.Language, duration time.Duration) {
			h.handleSessionEnd(sessID, finalCode, lang, duration)
		})
	}

	room.Join(client)
	defer func() {
		room.Leave(client)
	}()

	_, msg, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var init models.WSFrame
	if err := json.Unmarshal(msg, &init); err != nil || init.Type != "init" {
		_ = conn.WriteJSON(errFrame("expected init"))
		return
	}
	var initReq models.InitRequest
	b, _ := json.Marshal(init.Data)
	_ = json.Unmarshal(b, &initReq)

	// Set preferred language for the room (optional)
	if initReq.Language != "" {
		room.SetLanguage(initReq.Language)
	}
	doc, lang := room.Snapshot()
	if doc.Text == "" {
		spec, _, _, _, specErr := h.runner.LangSpecPublic(lang)
		if specErr == nil && spec.ExampleTemplate != "" {
			doc = room.BootstrapDoc(spec.ExampleTemplate)
		}
	}
	_ = conn.WriteJSON(models.WSFrame{
		Type: "init",
		Data: models.InitResponse{
			SessionID: sessionID,
			Doc:       doc,
			Language:  lang,
		},
	})

	room.ReplayRunHistory(client)

	// Event loop
	for {
		var frame models.WSFrame
		if err := conn.ReadJSON(&frame); err != nil {
			return
		}

		switch frame.Type {
		case "edit":
			var e models.Edit
			marshal(frame.Data, &e)
			ok, newDoc, applyErr := room.ApplyEdit(e)
			if !ok {
				errType := mapOTError(applyErr)
				_ = conn.WriteJSON(models.WSFrame{Type: "error", Data: errType})
				_ = conn.WriteJSON(models.WSFrame{Type: "doc", Data: newDoc})
				continue
			}
			// broadcast updated authoritative doc to all peers
			room.Broadcast(client, models.WSFrame{Type: "doc", Data: newDoc})
			// echo doc back to sender (ack)
			_ = conn.WriteJSON(models.WSFrame{Type: "doc", Data: newDoc})

		case "cursor":
			var c models.Cursor
			marshal(frame.Data, &c)
			room.Broadcast(client, models.WSFrame{Type: "cursor", Data: c})

		case "chat":
			var ch models.Chat
			marshal(frame.Data, &ch)
			room.Broadcast(client, models.WSFrame{Type: "chat", Data: ch})

		case "language":
			var langChange models.LanguageChange
			marshal(frame.Data, &langChange)
			if langChange.Language == "" {
				continue
			}
			room.SetLanguage(langChange.Language)
			room.Broadcast(client, models.WSFrame{Type: "language", Data: langChange.Language})
			_ = conn.WriteJSON(models.WSFrame{Type: "language", Data: langChange.Language})

		case "run":
			var run models.RunCmd
			marshal(frame.Data, &run)
			room.BeginRun()
			go h.runInSandbox(room, run)

		case "end_session":
			room.BroadcastAll(models.WSFrame{Type: "session_ended", Data: map[string]string{"reason": "partner_left"}})
			room.EndSessionNow()
			return

		default:
			_ = conn.WriteJSON(errFrame("unknown_type"))
		}
	}
}

func (h *Handlers) runInSandbox(room *session.Room, run models.RunCmd) {
	limits := exec.SandboxLimits{
		WallTime: 10 * time.Second,
		MemoryB:  512 * 1024 * 1024,
		NanoCPUs: 1_000_000_000,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	frames, runErr := h.runner.RunStream(ctx, run.Language, run.Code, limits)
	if runErr != nil && !errors.Is(runErr, exec.ErrDockerUnavailable) {
		h.log.Error("sandbox run failed", "language", run.Language, "error", runErr.Error())
	}
	if len(frames) == 0 && runErr != nil {
		room.RecordRunFrame(models.WSFrame{Type: "error", Data: runErr.Error()})
		return
	}
	for _, frame := range frames {
		room.RecordRunFrame(frame)
	}
}

func marshal(in any, out any) { b, _ := json.Marshal(in); _ = json.Unmarshal(b, out) }

func errFrame(msg string) models.WSFrame { return models.WSFrame{Type: "error", Data: msg} }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func mapOTError(err error) string {
	if err == nil {
		return "ot_error"
	}
	if errors.Is(err, text.ErrTransformTooOld) || errors.Is(err, text.ErrTransformSkipped) {
		return "version_mismatch"
	}
	if errors.Is(err, text.ErrTransformOOB) || errors.Is(err, text.ErrTransformNegDelete) {
		return "invalid_range"
	}
	if errors.Is(err, text.ErrTransformTooLong) {
		return "transform_too_long"
	}
	if err.Error() == "version_mismatch" || err.Error() == "invalid_range" {
		return err.Error()
	}
	return err.Error()
}

func (h *Handlers) handleSessionEnd(sessionID string, finalCode string, lang models.Language, duration time.Duration) {
	h.log.Info("Session ended", "sessionID", sessionID, "duration", duration.Seconds())

	if err := h.roomManager.MarkRoomAsEnded(sessionID); err != nil {
		h.log.Error("Failed to mark room as ended", "sessionID", sessionID, "error", err.Error())
	}

	roomInfo, err := h.roomManager.GetRoomStatus(sessionID)
	if err != nil {
		h.log.Error("Failed to get room info for ended session", "sessionID", sessionID, "error", err.Error())
		return
	}

	event := models.SessionEndedEvent{
		MatchID:     sessionID,
		User1:       roomInfo.User1,
		User2:       roomInfo.User2,
		Category:    roomInfo.Category,
		Difficulty:  roomInfo.Difficulty,
		Language:    string(lang),
		FinalCode:   finalCode,
		EndedAt:     time.Now().Format(time.RFC3339),
		DurationSec: int(duration.Seconds()),
		RerollsUsed: 1 - roomInfo.RerollsRemaining, // Initial rerolls (1) minus remaining
	}

	if roomInfo.CreatedAt != "" {
		event.StartedAt = roomInfo.CreatedAt
	}

	if roomInfo.Question != nil {
		event.QuestionID = roomInfo.Question.ID
		event.QuestionTitle = roomInfo.Question.Title
	}

	if err := h.roomManager.PublishSessionEnded(event); err != nil {
		h.log.Error("Failed to publish session ended event", "sessionID", sessionID, "error", err.Error())
	}

	h.hub.Delete(sessionID)
	h.log.Info("Cleaned up room from hub", "sessionID", sessionID)
}
