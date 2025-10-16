package session

import (
	"errors"
	"sync"

	"github.com/Jeffail/leaps/lib/text"

	"collab/internal/models"
)

// Room holds the authoritative document state and connected clients for a session.
type Room struct {
	ID       string
	mu       sync.Mutex
	clients  map[*Client]struct{}
	doc      models.DocState
	language models.Language
	otConf   text.OTBufferConfig
	otBuffer *text.OTBuffer
}

const otRetentionSeconds int64 = 60

func NewRoom(id string) *Room {
	cfg := text.NewOTBufferConfig()
	buf := text.NewOTBuffer("", cfg)
	buf.Version = 0

	return &Room{
		ID:       id,
		clients:  make(map[*Client]struct{}),
		doc:      models.DocState{Text: "", Version: 0},
		language: models.LangPython,
		otConf:   cfg,
		otBuffer: buf,
	}
}

func (r *Room) Join(c *Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[c] = struct{}{}
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
	return len(r.clients)
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

func (r *Room) resetOTBufferLocked() {
	buf := text.NewOTBuffer(r.doc.Text, r.otConf)
	buf.Version = int(r.doc.Version)
	r.otBuffer = buf
}
