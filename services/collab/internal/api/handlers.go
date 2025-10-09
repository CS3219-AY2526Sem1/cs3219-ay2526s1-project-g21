package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"collab/internal/exec"
	"collab/internal/format"
	"collab/internal/models"
	"collab/internal/session"
	"collab/internal/utils"
)

type Handlers struct {
	log    *utils.Logger
	runner *exec.Runner
	hub    *session.Hub
}

func NewHandlers(log *utils.Logger) *Handlers {
	return &Handlers{
		log:    log,
		runner: exec.NewRunner(),
		hub:    session.NewHub(),
	}
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok"))
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	client := session.NewClient(conn)
	room := h.hub.GetOrCreate(sessionID)
	room.Join(client)
	defer func() {
		if left := room.Leave(client); left == 0 {
			h.hub.Delete(sessionID)
		}
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
			ok, newDoc := room.ApplyEdit(e)
			if !ok {
				_ = conn.WriteJSON(models.WSFrame{Type: "error", Data: "version_mismatch"})
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
			go h.runInSandbox(conn, run) // stream back stdout/stderr/exit

		default:
			_ = conn.WriteJSON(errFrame("unknown_type"))
		}
	}
}

func (h *Handlers) runInSandbox(conn *websocket.Conn, run models.RunCmd) {
	limits := exec.SandboxLimits{
		WallTime: 10 * time.Second,
		MemoryB:  512 * 1024 * 1024,
		NanoCPUs: 1_000_000_000,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	// _, image, fileName, cmds, err := h.runner.LangSpecPublic(run.Language)
	_, image, _, cmds, err := h.runner.LangSpecPublic(run.Language)
	if err != nil {
		_ = conn.WriteJSON(errFrame("unsupported_language"))
		return
	}

	sbx, err := exec.NewSandbox(image, limits)
	if err != nil {
		_ = conn.WriteJSON(errFrame("sandbox_error"))
		return
	}

	exit, timedOut, runErr := sbx.Run(
		ctx,
		fileNameFromLang(run.Language),
		[]byte(run.Code),
		cmds,
		func(p []byte) { _ = conn.WriteJSON(models.WSFrame{Type: "stdout", Data: string(p)}) },
		func(p []byte) { _ = conn.WriteJSON(models.WSFrame{Type: "stderr", Data: string(p)}) },
	)
	if runErr != nil {
		h.log.Error("sandbox run failed", "language", run.Language, "error", runErr.Error())
		_ = conn.WriteJSON(errFrame("sandbox_error: " + runErr.Error()))
	}
	_ = conn.WriteJSON(models.WSFrame{Type: "exit", Data: map[string]any{"code": exit, "timedOut": timedOut}})
}

func marshal(in any, out any) { b, _ := json.Marshal(in); _ = json.Unmarshal(b, out) }

func errFrame(msg string) models.WSFrame { return models.WSFrame{Type: "error", Data: msg} }

func fileNameFromLang(l models.Language) string {
	switch l {
	case models.LangPython:
		return "main.py"
	case models.LangJava:
		return "Main.java"
	default:
		return "main.cpp"
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
