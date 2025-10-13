package session

import (
	"sync"

	"collab/internal/models"
)

// Room holds the authoritative document state and connected clients for a session.
type Room struct {
	ID       string
	mu       sync.Mutex
	clients  map[*Client]struct{}
	doc      models.DocState
	language models.Language
}

func NewRoom(id string) *Room {
	return &Room{
		ID:       id,
		clients:  make(map[*Client]struct{}),
		doc:      models.DocState{Text: "", Version: 0},
		language: models.LangPython,
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
	}
	return r.doc
}

func (r *Room) ApplyEdit(e models.Edit) (ok bool, newDoc models.DocState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.BaseVersion != r.doc.Version {
		return false, r.doc
	}
	if e.RangeStart < 0 || e.RangeEnd < e.RangeStart || e.RangeEnd > len(r.doc.Text) {
		return false, r.doc
	}
	text := r.doc.Text[:e.RangeStart] + e.Text + r.doc.Text[e.RangeEnd:]
	r.doc.Text = text
	r.doc.Version++
	return true, r.doc
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
