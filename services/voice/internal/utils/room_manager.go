package utils

import (
	"sync"

	"voice/internal/models"
)

// RoomManager manages voice chat rooms
type RoomManager struct {
	rooms map[string]*models.Room
	mu    sync.RWMutex
}

// NewRoomManager creates a new room manager
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*models.Room),
	}
}

// GetOrCreateRoom gets an existing room or creates a new one
func (rm *RoomManager) GetOrCreateRoom(roomID string) *models.Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if room, exists := rm.rooms[roomID]; exists {
		return room
	}

	room := models.NewRoom(roomID)
	rm.rooms[roomID] = room
	return room
}

// GetRoom gets an existing room
func (rm *RoomManager) GetRoom(roomID string) (*models.Room, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	room, exists := rm.rooms[roomID]
	return room, exists
}

// DeleteRoom deletes a room if it's empty
func (rm *RoomManager) DeleteRoom(roomID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if room, exists := rm.rooms[roomID]; exists {
		if room.GetUserCount() == 0 {
			delete(rm.rooms, roomID)
		}
	}
}
