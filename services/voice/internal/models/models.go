package models

import (
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

var (
	ErrUserNotFound = errors.New("user not found in room")
	ErrRoomNotFound = errors.New("room not found")
)

// RoomInfo represents match metadata from match service (stored in Redis)
type RoomInfo struct {
	MatchId    string `json:"matchId"`
	User1      string `json:"user1"`
	User2      string `json:"user2"`
	Category   string `json:"category"`
	Difficulty string `json:"difficulty"`
	Status     string `json:"status"` // "pending", "ready", "error"
	Token1     string `json:"token1,omitempty"`
	Token2     string `json:"token2,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

// Room represents an active voice chat session (We use this for in memory management)
type Room struct {
	ID          string
	Users       map[string]*User           // userID -> User
	Connections map[string]*websocket.Conn // userID -> WebSocket connection
	mu          sync.RWMutex
	CreatedAt   time.Time
}

// NewRoom creates a new voice chat room
func NewRoom(roomID string) *Room {
	return &Room{
		ID:          roomID,
		Users:       make(map[string]*User),
		Connections: make(map[string]*websocket.Conn),
		CreatedAt:   time.Now(),
	}
}

// AddUser adds a user to the room
func (r *Room) AddUser(user *User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Users[user.ID] = user
}

// RemoveUser removes a user from the room
func (r *Room) RemoveUser(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Users, userID)
}

// GetUser retrieves a user from the room
func (r *Room) GetUser(userID string) (*User, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, exists := r.Users[userID]
	return user, exists
}

// AddConn adds a WebSocket connection for a user
func (r *Room) AddConn(userID string, conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Connections[userID] = conn
}

// RemoveConn removes a WebSocket connection for a user
func (r *Room) RemoveConn(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if conn, exists := r.Connections[userID]; exists {
		if conn != nil {
			conn.Close()
		}
		delete(r.Connections, userID)
	}
}

// SendToUser sends a message to a specific user in the room
func (r *Room) SendToUser(userID string, msg interface{}) error {
	r.mu.RLock()
	conn, exists := r.Connections[userID]
	r.mu.RUnlock()

	if !exists {
		return ErrUserNotFound
	}

	return conn.WriteJSON(msg)
}

// BroadcastJSON broadcasts a JSON message to all users in the room
func (r *Room) BroadcastJSON(msg interface{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for userID, conn := range r.Connections {
		if err := conn.WriteJSON(msg); err != nil {
			// Log error but continue broadcasting to others
			_ = userID // suppress unused warning
		}
	}
}

// GetUserCount returns the number of users in the room
func (r *Room) GetUserCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Users)
}

// GetRoomStatus returns the current room status for broadcasting
func (r *Room) GetRoomStatus() *RoomStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]UserInfo, 0, len(r.Users))
	for _, user := range r.Users {
		users = append(users, UserInfo{
			ID:       user.ID,
			Username: user.Username,
			IsMuted:  user.IsMuted,
			IsDeaf:   user.IsDeaf,
			JoinedAt: user.JoinedAt,
		})
	}

	return &RoomStatus{
		Type:      "room-status",
		RoomID:    r.ID,
		Users:     users,
		UserCount: len(r.Users),
		Timestamp: time.Now(),
	}
}

// RoomStatus represents the status of a room for broadcasting to clients
type RoomStatus struct {
	Type      string     `json:"type"` // "room-status"
	RoomID    string     `json:"roomId"`
	Users     []UserInfo `json:"users"`
	UserCount int        `json:"userCount"`
	Timestamp time.Time  `json:"timestamp"`
}

// UserInfo represents user information for broadcasting (no sensitive data)
type UserInfo struct {
	ID       string    `json:"id"`
	Username string    `json:"username"`
	IsMuted  bool      `json:"isMuted"`
	IsDeaf   bool      `json:"isDeaf"`
	JoinedAt time.Time `json:"joinedAt"`
}

// User represents a user in a voice room
type User struct {
	ID       string                 `json:"id"`
	Username string                 `json:"username"`
	PeerConn *webrtc.PeerConnection `json:"-"`
	DataChan *webrtc.DataChannel    `json:"-"`
	IsMuted  bool                   `json:"isMuted"`
	IsDeaf   bool                   `json:"isDeaf"`
	JoinedAt time.Time              `json:"joinedAt"`
}

// SignalingMessage represents WebRTC signaling messages
type SignalingMessage struct {
	Type      string      `json:"type"` // "offer", "answer", "ice-candidate", "join", "leave", "mute", "unmute"
	From      string      `json:"from"`
	To        string      `json:"to"`
	RoomID    string      `json:"roomId"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// WebRTCConfig contains STUN/TURN server configuration
type WebRTCConfig struct {
	IceServers []IceServer `json:"iceServers"`
}

type IceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

// PresenceEvent is published to Redis for cross-instance presence sync
type PresenceEvent struct {
	Type       string    `json:"type"` // "user-joined", "user-left", "user-muted", "user-unmuted"
	RoomID     string    `json:"roomId"`
	UserID     string    `json:"userId"`
	Username   string    `json:"username,omitempty"`
	IsMuted    bool      `json:"isMuted"`
	Timestamp  time.Time `json:"timestamp"`
	InstanceID string    `json:"instanceId"` // Which service instance generated this event
}
