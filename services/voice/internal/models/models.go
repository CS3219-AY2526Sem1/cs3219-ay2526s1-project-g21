package models

import (
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
)

// Room represents a voice chat room
type Room struct {
	ID        string           `json:"id"`
	Users     map[string]*User `json:"users"`
	CreatedAt time.Time        `json:"createdAt"`
	mu        sync.RWMutex     `json:"-"`
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

// OfferData contains WebRTC offer information
type OfferData struct {
	SDP string `json:"sdp"`
}

// AnswerData contains WebRTC answer information
type AnswerData struct {
	SDP string `json:"sdp"`
}

// ICECandidateData contains ICE candidate information
type ICECandidateData struct {
	Candidate     string `json:"candidate"`
	SDPMLineIndex int    `json:"sdpMLineIndex"`
	SDPMid        string `json:"sdpMid"`
}

// JoinRoomRequest represents a request to join a voice room
type JoinRoomRequest struct {
	RoomID   string `json:"roomId"`
	UserID   string `json:"userId"`
	Username string `json:"username"`
}

// RoomStatus represents the current status of a voice room
type RoomStatus struct {
	ID        string     `json:"id"`
	UserCount int        `json:"userCount"`
	Users     []UserInfo `json:"users"`
	CreatedAt time.Time  `json:"createdAt"`
}

// UserInfo represents public user information in a room
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	IsMuted  bool   `json:"isMuted"`
	IsDeaf   bool   `json:"isDeaf"`
}

// WebRTCConfig contains WebRTC configuration
type WebRTCConfig struct {
	ICEServers []webrtc.ICEServer `json:"iceServers"`
}

// NewRoom creates a new voice room
func NewRoom(id string) *Room {
	return &Room{
		ID:        id,
		Users:     make(map[string]*User),
		CreatedAt: time.Now(),
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

// GetUser returns a user by ID
func (r *Room) GetUser(userID string) (*User, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, exists := r.Users[userID]
	return user, exists
}

// GetUsers returns all users in the room
func (r *Room) GetUsers() map[string]*User {
	r.mu.RLock()
	defer r.mu.RUnlock()
	users := make(map[string]*User)
	for id, user := range r.Users {
		users[id] = user
	}
	return users
}

// GetUserCount returns the number of users in the room
func (r *Room) GetUserCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Users)
}

// GetRoomStatus returns the current room status
func (r *Room) GetRoomStatus() RoomStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make([]UserInfo, 0, len(r.Users))
	for _, user := range r.Users {
		users = append(users, UserInfo{
			ID:       user.ID,
			Username: user.Username,
			IsMuted:  user.IsMuted,
			IsDeaf:   user.IsDeaf,
		})
	}

	return RoomStatus{
		ID:        r.ID,
		UserCount: len(r.Users),
		Users:     users,
		CreatedAt: r.CreatedAt,
	}
}
