package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Jeffail/leaps/lib/text"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"collab/internal/exec"
	"collab/internal/models"
	"collab/internal/room_management"
	"collab/internal/session"
	"collab/internal/utils"
)

type mockRunner struct {
	langSpecFn  func(models.Language) (models.LanguageSpec, string, string, [][]string, error)
	runOnceFn   func(context.Context, models.Language, string, exec.SandboxLimits) (exec.RunOutput, error)
	runStreamFn func(context.Context, models.Language, string, exec.SandboxLimits) ([]models.WSFrame, error)
}

func (m *mockRunner) LangSpecPublic(lang models.Language) (models.LanguageSpec, string, string, [][]string, error) {
	if m.langSpecFn != nil {
		return m.langSpecFn(lang)
	}
	return models.LanguageSpec{}, "", "", nil, nil
}

func (m *mockRunner) RunOnce(ctx context.Context, lang models.Language, code string, limits exec.SandboxLimits) (exec.RunOutput, error) {
	if m.runOnceFn != nil {
		return m.runOnceFn(ctx, lang, code, limits)
	}
	return exec.RunOutput{}, nil
}

func (m *mockRunner) RunStream(ctx context.Context, lang models.Language, code string, limits exec.SandboxLimits) ([]models.WSFrame, error) {
	if m.runStreamFn != nil {
		return m.runStreamFn(ctx, lang, code, limits)
	}
	return nil, nil
}

type mockRoomManager struct {
	validateFn func(string) (*models.RoomInfo, error)
	getFn      func(string) (*models.RoomInfo, error)
	rerollFn   func(string) (*models.RoomInfo, error)
	cb         func(string, *models.RoomInfo)
}

func (m *mockRoomManager) ValidateRoomAccess(token string) (*models.RoomInfo, error) {
	if m.validateFn != nil {
		return m.validateFn(token)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRoomManager) GetRoomStatus(id string) (*models.RoomInfo, error) {
	if m.getFn != nil {
		return m.getFn(id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRoomManager) RerollQuestion(id string) (*models.RoomInfo, error) {
	if m.rerollFn != nil {
		return m.rerollFn(id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockRoomManager) GetActiveRoomForUser(userId string) (*models.RoomInfo, error) {
	return nil, errors.New("not implemented")
}

func (m *mockRoomManager) PublishSessionEnded(event models.SessionEndedEvent) error {
	return nil
}

func (m *mockRoomManager) MarkRoomAsEnded(matchID string) error {
	return nil
}

func (m *mockRoomManager) GetInstanceID() string {
	return "abcd"
}

func (m *mockRoomManager) SetRoomUpdateCallback(callback func(matchId string, roomInfo *models.RoomInfo)) {
	m.cb = callback
}

func (m *mockRoomManager) SubscribeToRoomUpdates(ctx context.Context) {
	// no-op in tests
}

func newTestHandlers(runner runner, rm roomManager) *Handlers {
	logger := utils.NewLogger()
	return NewHandlersWithDeps(logger, runner, session.NewHub(), rm)
}

func addMatchID(ctx context.Context, matchID string) context.Context {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("matchId", matchID)
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}

func addSessionID(ctx context.Context, id string) context.Context {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}

func decodeBody(t *testing.T, body *bytes.Buffer, out interface{}) {
	t.Helper()
	if err := json.Unmarshal(body.Bytes(), out); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
}

func TestHealth(t *testing.T) {
	h := newTestHandlers(&mockRunner{}, &mockRoomManager{})
	rec := httptest.NewRecorder()
	h.Health(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Body.String() != "ok" {
		t.Fatalf("expected ok, got %q", rec.Body.String())
	}
}

func TestNewHandlersUsesDefaults(t *testing.T) {
	logger := utils.NewLogger()
	manager := room_management.NewRoomManager("localhost:0", "http://localhost")
	h := NewHandlers(logger, manager)
	if h == nil || h.roomManager == nil || h.runner == nil || h.hub == nil {
		t.Fatalf("expected handlers to initialize dependencies")
	}
}

func TestGetRoomStatusSuccess(t *testing.T) {
	rm := &mockRoomManager{
		validateFn: func(token string) (*models.RoomInfo, error) {
			if token != "token" {
				return nil, errors.New("bad token")
			}
			return &models.RoomInfo{MatchId: "m1"}, nil
		},
	}
	h := newTestHandlers(&mockRunner{}, rm)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/collab/room/m1", nil)
	req = req.WithContext(addMatchID(req.Context(), "m1"))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	h.GetRoomStatus(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp models.RoomInfo
	decodeBody(t, rec.Body, &resp)
	if resp.MatchId != "m1" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestGetRoomStatusErrors(t *testing.T) {
	h := newTestHandlers(&mockRunner{}, &mockRoomManager{
		validateFn: func(string) (*models.RoomInfo, error) { return nil, errors.New("auth error") },
	})

	missingID := httptest.NewRequest(http.MethodGet, "/api/v1/collab/room/", nil)
	rec := httptest.NewRecorder()
	h.GetRoomStatus(rec, missingID)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing matchId, got %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/collab/room/m1", nil)
	req = req.WithContext(addMatchID(req.Context(), "m1"))
	rec = httptest.NewRecorder()
	h.GetRoomStatus(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing auth, got %d", rec.Code)
	}

	req.Header.Set("Authorization", "Bearer token")
	rec = httptest.NewRecorder()
	h.GetRoomStatus(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for validation error, got %d", rec.Code)
	}
}

func TestRerollQuestionBranches(t *testing.T) {
	rm := &mockRoomManager{}
	hub := session.NewHub()
	runner := &mockRunner{}
	logger := utils.NewLogger()
	h := NewHandlersWithDeps(logger, runner, hub, rm)

	room := hub.GetOrCreate("m1")
	client := session.NewClient(nil)
	var mu sync.Mutex
	var broadcast []models.WSFrame
	client.SetSendHook(func(frame models.WSFrame) {
		mu.Lock()
		broadcast = append(broadcast, frame)
		mu.Unlock()
	})
	room.Join(client)

	validRoom := &models.RoomInfo{MatchId: "m1", Question: &models.Question{ID: 1}}
	rm.validateFn = func(string) (*models.RoomInfo, error) { return validRoom, nil }
	rm.rerollFn = func(id string) (*models.RoomInfo, error) {
		if id != "m1" {
			return nil, errors.New("bad id")
		}
		return &models.RoomInfo{
			MatchId:          "m1",
			Question:         &models.Question{ID: 2},
			RerollsRemaining: 1,
		}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/collab/room/m1/reroll", nil)
	req = req.WithContext(addMatchID(req.Context(), "m1"))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	h.RerollQuestion(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	waitUntil(func() bool { mu.Lock(); defer mu.Unlock(); return len(broadcast) == 1 }, t)

	rm.rerollFn = func(string) (*models.RoomInfo, error) { return nil, room_management.ErrNoRerolls }
	rec = httptest.NewRecorder()
	h.RerollQuestion(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for no rerolls, got %d", rec.Code)
	}

	rm.rerollFn = func(string) (*models.RoomInfo, error) { return nil, room_management.ErrNoAlternativeQuestion }
	rec = httptest.NewRecorder()
	h.RerollQuestion(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for no alternative, got %d", rec.Code)
	}

	rm.rerollFn = func(string) (*models.RoomInfo, error) { return nil, errors.New("boom") }
	rec = httptest.NewRecorder()
	h.RerollQuestion(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for unknown error, got %d", rec.Code)
	}

	rm.validateFn = func(string) (*models.RoomInfo, error) { return &models.RoomInfo{MatchId: "other"}, nil }
	rec = httptest.NewRecorder()
	h.RerollQuestion(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when room mismatch, got %d", rec.Code)
	}
}

func TestRerollQuestionMissingMatchID(t *testing.T) {
	h := newTestHandlers(&mockRunner{}, &mockRoomManager{})
	rec := httptest.NewRecorder()
	h.RerollQuestion(rec, httptest.NewRequest(http.MethodPost, "/api/v1/collab/room//reroll", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing matchId, got %d", rec.Code)
	}
}

func TestRerollQuestionUnauthorized(t *testing.T) {
	rm := &mockRoomManager{
		validateFn: func(string) (*models.RoomInfo, error) { return nil, errors.New("bad token") },
	}
	h := newTestHandlers(&mockRunner{}, rm)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collab/room/m1/reroll", nil)
	req = req.WithContext(addMatchID(req.Context(), "m1"))
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()

	h.RerollQuestion(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when validation fails, got %d", rec.Code)
	}
}

func TestRerollQuestionInvalidHeader(t *testing.T) {
	h := newTestHandlers(&mockRunner{}, &mockRoomManager{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collab/room/m1/reroll", nil)
	req = req.WithContext(addMatchID(req.Context(), "m1"))
	req.Header.Set("Authorization", "Token abc")
	rec := httptest.NewRecorder()

	h.RerollQuestion(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad header, got %d", rec.Code)
	}
}

func TestRerollQuestionMissingAuth(t *testing.T) {
	h := newTestHandlers(&mockRunner{}, &mockRoomManager{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collab/room/m1/reroll", nil)
	req = req.WithContext(addMatchID(req.Context(), "m1"))
	rec := httptest.NewRecorder()

	h.RerollQuestion(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when auth header missing, got %d", rec.Code)
	}
}

func TestListLanguages(t *testing.T) {
	runner := &mockRunner{
		langSpecFn: func(lang models.Language) (models.LanguageSpec, string, string, [][]string, error) {
			switch lang {
			case models.LangPython:
				return models.LanguageSpec{Name: lang}, "", "", nil, nil
			case models.LangCPP:
				return models.LanguageSpec{Name: lang}, "", "", nil, nil
			default:
				return models.LanguageSpec{}, "", "", nil, errors.New("missing")
			}
		},
	}
	h := newTestHandlers(runner, &mockRoomManager{})
	rec := httptest.NewRecorder()
	h.ListLanguages(rec, httptest.NewRequest(http.MethodGet, "/api/v1/collab/languages", nil))

	var resp []models.LanguageSpec
	decodeBody(t, rec.Body, &resp)
	if len(resp) != 2 {
		t.Fatalf("expected 2 language specs, got %d", len(resp))
	}
}

func TestFormatCode(t *testing.T) {
	h := newTestHandlers(&mockRunner{}, &mockRoomManager{})
	body := bytes.NewBufferString(`{"language":"python","code":"x"}`)
	rec := httptest.NewRecorder()
	h.FormatCode(rec, httptest.NewRequest(http.MethodPost, "/api/v1/collab/format", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.FormatCode(rec, httptest.NewRequest(http.MethodPost, "/api/v1/collab/format", bytes.NewBufferString(`bad-json`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad json, got %d", rec.Code)
	}
}

func TestFormatCodeContextError(t *testing.T) {
	h := newTestHandlers(&mockRunner{}, &mockRoomManager{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/collab/format", bytes.NewBufferString(`{"language":"python","code":"x"}`))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.FormatCode(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when formatter fails, got %d", rec.Code)
	}
}

func TestRunOnceHandler(t *testing.T) {
	runner := &mockRunner{
		runOnceFn: func(ctx context.Context, lang models.Language, code string, limits exec.SandboxLimits) (exec.RunOutput, error) {
			return exec.RunOutput{Stdout: "out", Exit: 0}, nil
		},
	}
	rm := &mockRoomManager{}
	h := newTestHandlers(runner, rm)

	body := bytes.NewBufferString(`{"language":"python","code":"print()"}`)
	rec := httptest.NewRecorder()
	h.RunOnce(rec, httptest.NewRequest(http.MethodPost, "/api/v1/collab/run", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	runner.runOnceFn = func(context.Context, models.Language, string, exec.SandboxLimits) (exec.RunOutput, error) {
		return exec.RunOutput{}, exec.ErrDockerUnavailable
	}
	rec = httptest.NewRecorder()
	h.RunOnce(rec, httptest.NewRequest(http.MethodPost, "/api/v1/collab/run", bytes.NewBufferString(`{"language":"python"}`)))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	runner.runOnceFn = func(context.Context, models.Language, string, exec.SandboxLimits) (exec.RunOutput, error) {
		return exec.RunOutput{}, errors.New("fail")
	}
	rec = httptest.NewRecorder()
	h.RunOnce(rec, httptest.NewRequest(http.MethodPost, "/api/v1/collab/run", bytes.NewBufferString(`{"language":"python"}`)))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.RunOnce(rec, httptest.NewRequest(http.MethodPost, "/api/v1/collab/run", bytes.NewBufferString(`bad`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad json, got %d", rec.Code)
	}
}

func TestRunInSandboxError(t *testing.T) {
	runner := &mockRunner{
		runStreamFn: func(context.Context, models.Language, string, exec.SandboxLimits) ([]models.WSFrame, error) {
			return nil, errors.New("boom")
		},
	}
	h := newTestHandlers(runner, &mockRoomManager{})
	room := session.NewRoom("id")
	client := session.NewClient(nil)
	var mu sync.Mutex
	var frames []models.WSFrame
	client.SetSendHook(func(frame models.WSFrame) {
		mu.Lock()
		frames = append(frames, frame)
		mu.Unlock()
	})
	room.Join(client)

	h.runInSandbox(room, models.RunCmd{Language: models.LangPython, Code: "print"})

	mu.Lock()
	defer mu.Unlock()
	if len(frames) != 1 || frames[0].Type != "error" {
		t.Fatalf("expected error frame, got %#v", frames)
	}
}

func TestRunInSandboxDockerUnavailable(t *testing.T) {
	runner := &mockRunner{
		runStreamFn: func(context.Context, models.Language, string, exec.SandboxLimits) ([]models.WSFrame, error) {
			return nil, exec.ErrDockerUnavailable
		},
	}
	h := newTestHandlers(runner, &mockRoomManager{})
	room := session.NewRoom("id")
	client := session.NewClient(nil)
	var frames []models.WSFrame
	client.SetSendHook(func(frame models.WSFrame) { frames = append(frames, frame) })
	room.Join(client)

	h.runInSandbox(room, models.RunCmd{Language: models.LangPython})
	if len(frames) != 1 || frames[0].Type != "error" {
		t.Fatalf("expected error frame for docker unavailable, got %#v", frames)
	}
}

func TestCollabWSFlow(t *testing.T) {
	room := &models.RoomInfo{MatchId: "room1", User1: "u1"}
	runner := &mockRunner{
		langSpecFn: func(models.Language) (models.LanguageSpec, string, string, [][]string, error) {
			return models.LanguageSpec{
				Name:            models.LangPython,
				ExampleTemplate: "print('hello')\n",
			}, "", "", nil, nil
		},
		runStreamFn: func(context.Context, models.Language, string, exec.SandboxLimits) ([]models.WSFrame, error) {
			return []models.WSFrame{
				{Type: "stdout", Data: "line"},
				{Type: "exit", Data: map[string]any{"code": 0, "timedOut": false}},
			}, nil
		},
	}
	rm := &mockRoomManager{
		validateFn: func(token string) (*models.RoomInfo, error) {
			if token != "valid" {
				return nil, errors.New("invalid token")
			}
			return room, nil
		},
	}
	logger := utils.NewLogger()
	h := NewHandlersWithDeps(logger, runner, session.NewHub(), rm)

	router := chi.NewRouter()
	router.Get("/ws/session/{id}", h.CollabWS)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/session/room1?token=valid"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(models.WSFrame{Type: "init", Data: map[string]any{"language": "python"}}); err != nil {
		t.Fatalf("send init: %v", err)
	}

	var frame models.WSFrame
	if err := conn.ReadJSON(&frame); err != nil {
		t.Fatalf("read init response: %v", err)
	}
	if frame.Type != "init" {
		t.Fatalf("expected init response, got %q", frame.Type)
	}
	initData, _ := json.Marshal(frame.Data)
	var initResp models.InitResponse
	_ = json.Unmarshal(initData, &initResp)

	edit := models.WSFrame{
		Type: "edit",
		Data: models.Edit{
			BaseVersion: initResp.Doc.Version,
			RangeStart:  0,
			RangeEnd:    0,
			Text:        "X",
		},
	}
	if err := conn.WriteJSON(edit); err != nil {
		t.Fatalf("send edit: %v", err)
	}
	if err := conn.ReadJSON(&frame); err != nil {
		t.Fatalf("read doc ack: %v", err)
	}
	if frame.Type != "doc" {
		t.Fatalf("expected doc response, got %q", frame.Type)
	}

	conn.WriteJSON(models.WSFrame{Type: "cursor", Data: models.Cursor{UserID: "u1", Pos: 1}})
	conn.WriteJSON(models.WSFrame{Type: "chat", Data: models.Chat{UserID: "u1", Message: "hi"}})

	if err := conn.WriteJSON(models.WSFrame{Type: "language", Data: models.LanguageChange{Language: models.LangPython}}); err != nil {
		t.Fatalf("send language: %v", err)
	}
	if err := conn.ReadJSON(&frame); err != nil {
		t.Fatalf("read language ack: %v", err)
	}
	if frame.Type != "language" {
		t.Fatalf("expected language ack, got %q", frame.Type)
	}
	_ = conn.WriteJSON(models.WSFrame{Type: "language", Data: models.LanguageChange{Language: ""}})

	if err := conn.WriteJSON(models.WSFrame{Type: "run", Data: models.RunCmd{Language: models.LangPython, Code: "print()"}}); err != nil {
		t.Fatalf("send run: %v", err)
	}
	if err := conn.ReadJSON(&frame); err != nil {
		t.Fatalf("read run reset: %v", err)
	}
	if frame.Type != "run_reset" {
		t.Fatalf("expected run_reset, got %q", frame.Type)
	}
	if err := conn.ReadJSON(&frame); err != nil || frame.Type != "stdout" {
		t.Fatalf("expected stdout frame, got %#v err=%v", frame, err)
	}
	if err := conn.ReadJSON(&frame); err != nil || frame.Type != "exit" {
		t.Fatalf("expected exit frame, got %#v err=%v", frame, err)
	}

	// Send invalid edit to trigger OT error handling.
	if err := conn.WriteJSON(models.WSFrame{Type: "edit", Data: models.Edit{BaseVersion: 999, RangeStart: 5, RangeEnd: 3}}); err != nil {
		t.Fatalf("send invalid edit: %v", err)
	}
	if err := conn.ReadJSON(&frame); err != nil || frame.Type != "error" {
		t.Fatalf("expected error frame after invalid edit, got %#v err=%v", frame, err)
	}
	if err := conn.ReadJSON(&frame); err != nil || frame.Type != "doc" {
		t.Fatalf("expected doc sync after error, got %#v err=%v", frame, err)
	}

	if err := conn.WriteJSON(models.WSFrame{Type: "unknown"}); err != nil {
		t.Fatalf("send unknown: %v", err)
	}
	if err := conn.ReadJSON(&frame); err != nil || frame.Type != "error" {
		t.Fatalf("expected error frame, got %#v err=%v", frame, err)
	}

	// Connect second client to ensure room reuse.
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial second websocket: %v", err)
	}
	defer conn2.Close()
	if err := conn2.WriteJSON(models.WSFrame{Type: "init", Data: map[string]any{"language": "python"}}); err != nil {
		t.Fatalf("send init 2: %v", err)
	}
	if err := conn2.ReadJSON(&frame); err != nil || frame.Type != "init" {
		t.Fatalf("expected init response for second client, got %#v err=%v", frame, err)
	}

	// Third client should be rejected because room already has 2 participants.
	conn3, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial third websocket: %v", err)
	}
	defer conn3.Close()
	if err := conn3.ReadJSON(&frame); err != nil || frame.Type != "error" || frame.Data != "room_full" {
		t.Fatalf("expected room_full error, got %#v err=%v", frame, err)
	}
}

func TestCollabWSMissingToken(t *testing.T) {
	h := newTestHandlers(&mockRunner{}, &mockRoomManager{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws/session/room1", nil)
	req = req.WithContext(addSessionID(req.Context(), "room1"))

	h.CollabWS(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestCollabWSValidateError(t *testing.T) {
	rm := &mockRoomManager{validateFn: func(string) (*models.RoomInfo, error) { return nil, errors.New("bad") }}
	h := newTestHandlers(&mockRunner{}, rm)
	req := httptest.NewRequest(http.MethodGet, "/ws/session/room1?token=bad", nil)
	req = req.WithContext(addSessionID(req.Context(), "room1"))
	rec := httptest.NewRecorder()

	h.CollabWS(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when validation fails, got %d", rec.Code)
	}
}

func TestCollabWSRoomMismatch(t *testing.T) {
	rm := &mockRoomManager{validateFn: func(string) (*models.RoomInfo, error) {
		return &models.RoomInfo{MatchId: "other"}, nil
	}}
	h := newTestHandlers(&mockRunner{}, rm)
	req := httptest.NewRequest(http.MethodGet, "/ws/session/room1?token=ok", nil)
	req = req.WithContext(addSessionID(req.Context(), "room1"))
	rec := httptest.NewRecorder()

	h.CollabWS(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for mismatch, got %d", rec.Code)
	}
}

func TestCollabWSInvalidInitFrame(t *testing.T) {
	runner := &mockRunner{
		langSpecFn: func(models.Language) (models.LanguageSpec, string, string, [][]string, error) {
			return models.LanguageSpec{Name: models.LangPython}, "", "", nil, nil
		},
	}
	rm := &mockRoomManager{validateFn: func(string) (*models.RoomInfo, error) {
		return &models.RoomInfo{MatchId: "room1"}, nil
	}}
	h := NewHandlersWithDeps(utils.NewLogger(), runner, session.NewHub(), rm)

	router := chi.NewRouter()
	router.Get("/ws/session/{id}", h.CollabWS)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/session/room1?token=ok"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(models.WSFrame{Type: "noop"}); err != nil {
		t.Fatalf("send invalid init: %v", err)
	}
	var frame models.WSFrame
	if err := conn.ReadJSON(&frame); err != nil || frame.Type != "error" || frame.Data != "expected init" {
		t.Fatalf("expected init error, got %#v err=%v", frame, err)
	}
}

func TestCollabWSUpgradeError(t *testing.T) {
	rm := &mockRoomManager{validateFn: func(string) (*models.RoomInfo, error) {
		return &models.RoomInfo{MatchId: "room1"}, nil
	}}
	h := newTestHandlers(&mockRunner{}, rm)
	req := httptest.NewRequest(http.MethodGet, "/ws/session/room1?token=ok", nil)
	req = req.WithContext(addSessionID(req.Context(), "room1"))
	rec := httptest.NewRecorder()

	h.CollabWS(rec, req)
}

func TestCollabWSRoomFullBranch(t *testing.T) {
	runner := &mockRunner{
		langSpecFn: func(models.Language) (models.LanguageSpec, string, string, [][]string, error) {
			return models.LanguageSpec{Name: models.LangPython}, "", "", nil, nil
		},
	}
	rm := &mockRoomManager{validateFn: func(string) (*models.RoomInfo, error) {
		return &models.RoomInfo{MatchId: "roomfull"}, nil
	}}
	h := NewHandlersWithDeps(utils.NewLogger(), runner, session.NewHub(), rm)
	room := h.hub.GetOrCreate("roomfull")
	c1 := session.NewClient(nil)
	c1.SetSendHook(func(models.WSFrame) {})
	room.Join(c1)
	c2 := session.NewClient(nil)
	c2.SetSendHook(func(models.WSFrame) {})
	room.Join(c2)

	router := chi.NewRouter()
	router.Get("/ws/session/{id}", h.CollabWS)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/session/roomfull?token=ok"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	var frame models.WSFrame
	if err := conn.ReadJSON(&frame); err != nil || frame.Type != "error" || frame.Data != "room_full" {
		t.Fatalf("expected room_full error, got %#v err=%v", frame, err)
	}
}

func TestCollabWSReadMessageError(t *testing.T) {
	runner := &mockRunner{
		langSpecFn: func(models.Language) (models.LanguageSpec, string, string, [][]string, error) {
			return models.LanguageSpec{Name: models.LangPython}, "", "", nil, nil
		},
	}
	rm := &mockRoomManager{validateFn: func(string) (*models.RoomInfo, error) {
		return &models.RoomInfo{MatchId: "roomerr"}, nil
	}}
	h := NewHandlersWithDeps(utils.NewLogger(), runner, session.NewHub(), rm)

	router := chi.NewRouter()
	router.Get("/ws/session/{id}", h.CollabWS)
	server := httptest.NewServer(router)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/session/roomerr?token=ok"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	conn.Close()
	time.Sleep(50 * time.Millisecond)
}

func TestMapOTError(t *testing.T) {
	if got := mapOTError(nil); got != "ot_error" {
		t.Fatalf("expected default ot_error, got %q", got)
	}
	if got := mapOTError(text.ErrTransformTooOld); got != "version_mismatch" {
		t.Fatalf("expected version_mismatch, got %q", got)
	}
	if got := mapOTError(text.ErrTransformOOB); got != "invalid_range" {
		t.Fatalf("expected invalid_range, got %q", got)
	}
	if got := mapOTError(text.ErrTransformTooLong); got != "transform_too_long" {
		t.Fatalf("expected transform_too_long, got %q", got)
	}
	if got := mapOTError(text.ErrTransformNegDelete); got != "invalid_range" {
		t.Fatalf("expected invalid_range for negative delete, got %q", got)
	}
	if got := mapOTError(text.ErrTransformSkipped); got != "version_mismatch" {
		t.Fatalf("expected version_mismatch for skipped transform, got %q", got)
	}
	if got := mapOTError(errors.New("other")); got != "other" {
		t.Fatalf("expected passthrough error, got %q", got)
	}
}

func waitUntil(cond func() bool, t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met")
}
