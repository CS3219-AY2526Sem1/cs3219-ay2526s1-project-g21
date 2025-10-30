package session

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Jeffail/leaps/lib/text"
	"github.com/gorilla/websocket"

	"collab/internal/models"
)

type frameCapture struct {
	frames []models.WSFrame
}

func newFrameCapture() *frameCapture { return &frameCapture{} }

func (c *frameCapture) hook(frame models.WSFrame) { c.frames = append(c.frames, frame) }

func (c *frameCapture) list() []models.WSFrame {
	out := make([]models.WSFrame, len(c.frames))
	copy(out, c.frames)
	return out
}

func TestClientSendWithHook(t *testing.T) {
	client := NewClient(nil)
	capture := newFrameCapture()
	client.SetSendHook(capture.hook)

	frame := models.WSFrame{Type: "ping"}
	client.Send(frame)

	got := capture.list()
	if len(got) != 1 || got[0].Type != "ping" {
		t.Fatalf("expected frame captured, got %#v", got)
	}
}

func TestClientSendWithoutConnDoesNotPanic(t *testing.T) {
	client := NewClient(nil)
	client.Send(models.WSFrame{Type: "noop"})
}

func TestClientSendWritesToConn(t *testing.T) {
	upgrader := websocket.Upgrader{}
	received := make(chan models.WSFrame, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var frame models.WSFrame
		if err := conn.ReadJSON(&frame); err == nil {
			received <- frame
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	client := NewClient(conn)
	client.Send(models.WSFrame{Type: "ping"})

	select {
	case frame := <-received:
		if frame.Type != "ping" {
			t.Fatalf("unexpected frame: %#v", frame)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected frame to be received")
	}
}

func TestRoomJoinLeaveAndSnapshot(t *testing.T) {
	room := NewRoom("room")
	if count := room.GetClientCount(); count != 0 {
		t.Fatalf("expected empty room, got %d", count)
	}

	c1 := NewClient(nil)
	c2 := NewClient(nil)
	room.Join(c1)
	room.Join(c2)
	if count := room.GetClientCount(); count != 2 {
		t.Fatalf("expected 2 clients, got %d", count)
	}

	doc, lang := room.Snapshot()
	if doc.Text != "" || doc.Version != 0 {
		t.Fatalf("unexpected doc snapshot: %#v", doc)
	}
	if lang != models.LangPython {
		t.Fatalf("expected default language python, got %s", lang)
	}

	room.SetLanguage(models.LangJava)
	_, lang = room.Snapshot()
	if lang != models.LangJava {
		t.Fatalf("expected language java, got %s", lang)
	}

	if left := room.Leave(c1); left != 1 {
		t.Fatalf("expected 1 client after leave, got %d", left)
	}
	if left := room.Leave(c2); left != 0 {
		t.Fatalf("expected empty room, got %d", left)
	}
}

func TestRoomBootstrapDoc(t *testing.T) {
	room := NewRoom("session")
	state := room.BootstrapDoc("template")
	if state.Text != "template" {
		t.Fatalf("expected template text, got %q", state.Text)
	}
	if state.Version != 1 {
		t.Fatalf("expected version 1, got %d", state.Version)
	}

	state = room.BootstrapDoc("ignored")
	if state.Text != "template" || state.Version != 1 {
		t.Fatalf("bootstrap should not overwrite existing doc, got %#v", state)
	}
}

func TestRoomApplyEditSuccess(t *testing.T) {
	room := NewRoom("r")
	ok, doc, err := room.ApplyEdit(models.Edit{
		BaseVersion: 0,
		RangeStart:  0,
		RangeEnd:    0,
		Text:        "hello",
	})
	if err != nil || !ok {
		t.Fatalf("expected successful edit, ok=%v err=%v", ok, err)
	}
	if doc.Text != "hello" || doc.Version != 1 {
		t.Fatalf("unexpected doc after edit: %#v", doc)
	}
}

func TestRoomApplyEditVersionMismatch(t *testing.T) {
	room := NewRoom("r")
	ok, _, err := room.ApplyEdit(models.Edit{
		BaseVersion: 5,
		RangeStart:  0,
		RangeEnd:    0,
		Text:        "data",
	})
	if err == nil || ok {
		t.Fatalf("expected version mismatch error")
	}
	if err.Error() != "version_mismatch" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoomApplyEditInvalidRange(t *testing.T) {
	room := NewRoom("r")
	ok, _, err := room.ApplyEdit(models.Edit{
		BaseVersion: 0,
		RangeStart:  5,
		RangeEnd:    3,
		Text:        "x",
	})
	if err == nil || ok {
		t.Fatalf("expected invalid range error")
	}
	if err.Error() != "invalid_range" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoomApplyEditOTError(t *testing.T) {
	room := NewRoom("r")
	room.BootstrapDoc("hi")
	ok, _, err := room.ApplyEdit(models.Edit{
		BaseVersion: 1,
		RangeStart:  10,
		RangeEnd:    10,
		Text:        "x",
	})
	if err == nil || ok {
		t.Fatalf("expected OT error for out-of-bounds edit")
	}
}

func TestRoomApplyEditFlushError(t *testing.T) {
	room := NewRoom("flush")
	room.BootstrapDoc("abcd")
	room.otBuffer.Unapplied = append(room.otBuffer.Unapplied, text.OTransform{
		Version:  room.otBuffer.Version + 1,
		Position: 10,
		Insert:   "x",
	})

	ok, _, err := room.ApplyEdit(models.Edit{
		BaseVersion: room.doc.Version,
		RangeStart:  0,
		RangeEnd:    0,
	})
	if err == nil || ok {
		t.Fatalf("expected flush error, got ok=%v err=%v", ok, err)
	}
}

func TestRoomBroadcast(t *testing.T) {
	room := NewRoom("r")
	frame := models.WSFrame{Type: "chat", Data: "hello"}

	c1 := NewClient(nil)
	cap1 := newFrameCapture()
	c1.SetSendHook(cap1.hook)
	c2 := NewClient(nil)
	cap2 := newFrameCapture()
	c2.SetSendHook(cap2.hook)
	sender := NewClient(nil)
	sender.SetSendHook(func(models.WSFrame) { t.Fatal("sender should not receive broadcast") })

	room.Join(c1)
	room.Join(c2)
	room.Join(sender)

	room.Broadcast(sender, frame)

	if got := cap1.list(); len(got) != 1 || got[0].Type != "chat" {
		t.Fatalf("client1 missing frame: %#v", got)
	}
	if got := cap2.list(); len(got) != 1 || got[0].Type != "chat" {
		t.Fatalf("client2 missing frame: %#v", got)
	}
}

func TestRoomBroadcastAll(t *testing.T) {
	room := NewRoom("r")
	frame := models.WSFrame{Type: "ping"}

	c1 := NewClient(nil)
	cap1 := newFrameCapture()
	c1.SetSendHook(cap1.hook)
	c2 := NewClient(nil)
	cap2 := newFrameCapture()
	c2.SetSendHook(cap2.hook)

	room.Join(c1)
	room.Join(c2)

	room.BroadcastAll(frame)

	if len(cap1.list()) != 1 || len(cap2.list()) != 1 {
		t.Fatalf("expected broadcast to all clients")
	}
}

func TestRoomRunHistoryLifecycle(t *testing.T) {
	room := NewRoom("r")

	c1 := NewClient(nil)
	cap1 := newFrameCapture()
	c1.SetSendHook(cap1.hook)
	room.Join(c1)

	c2 := NewClient(nil)
	cap2 := newFrameCapture()
	c2.SetSendHook(cap2.hook)
	room.Join(c2)

	room.BeginRun()
	runFrame := models.WSFrame{Type: "stdout", Data: "output"}
	room.RecordRunFrame(runFrame)

	if got := cap1.list(); len(got) != 2 || got[0].Type != "run_reset" || got[1] != runFrame {
		t.Fatalf("unexpected run history for client1: %#v", got)
	}
	if got := cap2.list(); len(got) != 2 || got[0].Type != "run_reset" || got[1] != runFrame {
		t.Fatalf("unexpected run history for client2: %#v", got)
	}

	replayClient := NewClient(nil)
	replayCap := newFrameCapture()
	replayClient.SetSendHook(replayCap.hook)
	room.ReplayRunHistory(replayClient)

	if got := replayCap.list(); len(got) != 2 || got[0].Type != "run_reset" || got[1] != runFrame {
		t.Fatalf("unexpected replay history: %#v", got)
	}
}

func TestHubLifecycle(t *testing.T) {
	hub := NewHub()
	roomA := hub.GetOrCreate("a")
	roomB := hub.GetOrCreate("a")
	if roomA != roomB {
		t.Fatalf("expected same room instance")
	}

	if _, ok := hub.Get("missing"); ok {
		t.Fatalf("expected missing room")
	}

	roomA.BootstrapDoc("code")
	if doc, ok := hub.GetDoc("a"); !ok || doc != "code" {
		t.Fatalf("expected cached doc, got %q ok=%v", doc, ok)
	}
	if doc, ok := hub.GetDoc("missing"); ok || doc != "" {
		t.Fatalf("expected missing doc, got %q ok=%v", doc, ok)
	}

	hub.Delete("a")
	if _, ok := hub.Get("a"); ok {
		t.Fatalf("expected room to be deleted")
	}
}
