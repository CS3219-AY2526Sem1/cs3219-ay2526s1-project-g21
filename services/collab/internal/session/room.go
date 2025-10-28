package session

import (
	"errors"
	"sync"
	"time"

	"github.com/Jeffail/leaps/lib/text"

	"collab/internal/models"
)

// Room holds the authoritative document state and connected clients for a session.
type Room struct {
	ID                string
	mu                sync.Mutex
	clients           map[*Client]struct{}
	doc               models.DocState
	language          models.Language
	otConf            text.OTBufferConfig
	otBuffer          *text.OTBuffer
	runHistory        []models.WSFrame
	startedAt         time.Time
	lastDisconnectAt  *time.Time
	allDisconnected   bool
	sessionEnded      bool
	sessionEndHandler func(sessionID string, finalCode string, language models.Language, duration time.Duration)
}

const otRetentionSeconds int64 = 60

func NewRoom(id string) *Room {
	cfg := text.NewOTBufferConfig()
	buf := text.NewOTBuffer("", cfg)
	buf.Version = 0

	return &Room{
		ID:              id,
		clients:         make(map[*Client]struct{}),
		doc:             models.DocState{Text: "", Version: 0},
		language:        models.LangPython,
		otConf:          cfg,
		otBuffer:        buf,
		startedAt:       time.Now(),
		allDisconnected: false,
	}
}

func (r *Room) SetSessionEndHandler(handler func(sessionID string, finalCode string, language models.Language, duration time.Duration)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessionEndHandler = handler
}

func (r *Room) Join(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c] = struct{}{}

	// Reset disconnect tracking if clients rejoin
	if r.allDisconnected {
		r.allDisconnected = false
		r.lastDisconnectAt = nil
	}
}

func (r *Room) GetClientCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients)
}

func (r *Room) Leave(c *Client) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, c)
	remaining := len(r.clients)

	// Track when all clients disconnect
	if remaining == 0 && !r.allDisconnected && !r.sessionEnded {
		now := time.Now()
		r.lastDisconnectAt = &now
		r.allDisconnected = true

		// Start a goroutine to check if session should end after 30 seconds
		go r.checkSessionEnd()
	}

	return remaining
}

func (r *Room) checkSessionEnd() {
	time.Sleep(30 * time.Second)

	r.mu.Lock()
	defer r.mu.Unlock()

	// If still no clients after 30 seconds, end the session
	if len(r.clients) == 0 && r.allDisconnected && r.sessionEndHandler != nil {
		duration := time.Since(r.startedAt)
		r.sessionEndHandler(r.ID, r.doc.Text, r.language, duration)
	}
}

func (r *Room) Snapshot() (models.DocState, models.Language) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.doc, r.language
}

func (r *Room) SetLanguage(l models.Language) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.language = l
}

func (r *Room) BootstrapDoc(template string) models.DocState {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.doc.Text == "" && template != "" {
		r.doc.Text = template
		r.doc.Version++
		r.resetOTBufferLocked()
	}
	return r.doc
}

func (r *Room) ApplyEdit(e models.Edit) (bool, models.DocState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if e.BaseVersion > r.doc.Version {
		return false, r.doc, errors.New("version_mismatch")
	}

	if e.RangeStart < 0 || e.RangeEnd < e.RangeStart {
		return false, r.doc, errors.New("invalid_range")
	}

	ot := text.OTransform{
		Version:  int(e.BaseVersion) + 1,
		Position: e.RangeStart,
		Delete:   e.RangeEnd - e.RangeStart,
		Insert:   e.Text,
	}

	if _, _, err := r.otBuffer.PushTransform(ot); err != nil {
		return false, r.doc, err
	}

	if _, err := r.otBuffer.FlushTransforms(&r.doc.Text, otRetentionSeconds); err != nil {
		return false, r.doc, err
	}

	r.doc.Version = int64(r.otBuffer.GetVersion())

	return true, r.doc, nil
}

func (r *Room) Broadcast(sender *Client, frame models.WSFrame) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for c := range r.clients {
		if c == sender {
			continue
		}
		c.Send(frame)
	}
}

func (r *Room) BroadcastAll(frame models.WSFrame) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for c := range r.clients {
		c.Send(frame)
	}
}

func (r *Room) EndSessionNow() {
	r.mu.Lock()
	if r.sessionEnded {
		r.mu.Unlock()
		return
	}
	handler := r.sessionEndHandler
	finalCode := r.doc.Text
	lang := r.language
	started := r.startedAt
	r.sessionEnded = true
	r.mu.Unlock()

	if handler != nil {
		duration := time.Since(started)
		go handler(r.ID, finalCode, lang, duration)
	}
}

func (r *Room) resetOTBufferLocked() {
	buf := text.NewOTBuffer(r.doc.Text, r.otConf)
	buf.Version = int(r.doc.Version)
	r.otBuffer = buf
}

func (r *Room) BeginRun() {
	r.mu.Lock()
	defer r.mu.Unlock()
	frame := models.WSFrame{Type: "run_reset"}
	r.runHistory = []models.WSFrame{frame}
	r.broadcastFrameLocked(frame)
}

func (r *Room) RecordRunFrame(frame models.WSFrame) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runHistory = append(r.runHistory, frame)
	r.broadcastFrameLocked(frame)
}

func (r *Room) ReplayRunHistory(c *Client) {
	r.mu.Lock()
	history := append([]models.WSFrame(nil), r.runHistory...)
	r.mu.Unlock()
	for _, frame := range history {
		c.Send(frame)
	}
}

func (r *Room) broadcastFrameLocked(frame models.WSFrame) {
	for client := range r.clients {
		client.Send(frame)
	}
}
