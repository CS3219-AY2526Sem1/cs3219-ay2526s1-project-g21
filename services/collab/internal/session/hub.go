package session

import "sync"

// Hub manages all active collaboration rooms.
type Hub struct {
	mu    sync.RWMutex
	rooms map[string]*Room
}

func NewHub() *Hub { return &Hub{rooms: make(map[string]*Room)} }

func (h *Hub) GetOrCreate(id string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[id]; ok {
		return r
	}
	r := NewRoom(id)
	h.rooms[id] = r
	return r
}

func (h *Hub) Delete(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms, id)
}

func (h *Hub) GetDoc(sessionID string) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	room, ok := h.rooms[sessionID]
	if !ok {
		return "", false
	}
	doc, _ := room.Snapshot()
	return doc.Text, true
}
