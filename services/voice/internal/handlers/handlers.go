package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"voice/internal/models"
	"voice/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type Handlers struct {
	roomManager  *utils.RoomManager
	upgrader     websocket.Upgrader
	webrtcConfig webrtc.Configuration
}

func NewHandlers() *Handlers {
	return &Handlers{
		roomManager:  utils.NewRoomManager(),
		upgrader:     websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
		webrtcConfig: utils.GetWebRTCConfig(),
	}
}

// Health check endpoint
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Get room status
func (h *Handlers) GetRoomStatus(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	if roomID == "" {
		http.Error(w, "roomId is required", http.StatusBadRequest)
		return
	}

	room, exists := h.roomManager.GetRoom(roomID)
	if !exists {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}

	status := room.GetRoomStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// Get WebRTC configuration
func (h *Handlers) GetWebRTCConfig(w http.ResponseWriter, r *http.Request) {
	config := models.WebRTCConfig{
		ICEServers: h.webrtcConfig.ICEServers,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// WebSocket handler for voice chat
func (h *Handlers) VoiceChatWS(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	if roomID == "" {
		http.Error(w, "roomId is required", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Get or create room
	room := h.roomManager.GetOrCreateRoom(roomID)

	// Handle WebSocket messages
	for {
		var msg models.SignalingMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		msg.Timestamp = time.Now()
		msg.RoomID = roomID

		// Handle different message types
		switch msg.Type {
		case "join":
			h.handleJoin(conn, room, &msg)
		case "leave":
			h.handleLeave(conn, room, &msg)
		case "offer":
			h.handleOffer(conn, room, &msg)
		case "answer":
			h.handleAnswer(conn, room, &msg)
		case "ice-candidate":
			h.handleICECandidate(conn, room, &msg)
		case "mute":
			h.handleMute(conn, room, &msg)
		case "unmute":
			h.handleUnmute(conn, room, &msg)
		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

func (h *Handlers) handleJoin(conn *websocket.Conn, room *models.Room, msg *models.SignalingMessage) {
	// Extract user info from message data
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid join message data")
		return
	}

	userID, _ := data["userId"].(string)
	username, _ := data["username"].(string)

	if userID == "" || username == "" {
		log.Printf("Missing user ID or username")
		return
	}

	// Create user
	user := &models.User{
		ID:       userID,
		Username: username,
		JoinedAt: time.Now(),
	}

	// Add user to room
	room.AddUser(user)

	// Send room status to all users
	h.broadcastRoomStatus(room)

	log.Printf("User %s joined room %s", username, room.ID)
}

func (h *Handlers) handleLeave(conn *websocket.Conn, room *models.Room, msg *models.SignalingMessage) {
	userID := msg.From
	room.RemoveUser(userID)

	// Send room status to remaining users
	h.broadcastRoomStatus(room)

	// Clean up empty rooms
	h.roomManager.DeleteRoom(room.ID)

	log.Printf("User %s left room %s", userID, room.ID)
}

func (h *Handlers) handleOffer(conn *websocket.Conn, room *models.Room, msg *models.SignalingMessage) {
	// Forward offer to target user
	h.forwardToUser(room, msg)
}

func (h *Handlers) handleAnswer(conn *websocket.Conn, room *models.Room, msg *models.SignalingMessage) {
	// Forward answer to target user
	h.forwardToUser(room, msg)
}

func (h *Handlers) handleICECandidate(conn *websocket.Conn, room *models.Room, msg *models.SignalingMessage) {
	// Forward ICE candidate to target user
	h.forwardToUser(room, msg)
}

func (h *Handlers) handleMute(conn *websocket.Conn, room *models.Room, msg *models.SignalingMessage) {
	userID := msg.From
	if user, exists := room.GetUser(userID); exists {
		user.IsMuted = true
		h.broadcastRoomStatus(room)
	}
}

func (h *Handlers) handleUnmute(conn *websocket.Conn, room *models.Room, msg *models.SignalingMessage) {
	userID := msg.From
	if user, exists := room.GetUser(userID); exists {
		user.IsMuted = false
		h.broadcastRoomStatus(room)
	}
}

func (h *Handlers) forwardToUser(room *models.Room, msg *models.SignalingMessage) {
	// In a real implementation, you would forward the message to the specific user
	// For now, we'll just log it
	log.Printf("Forwarding %s message from %s to %s in room %s", msg.Type, msg.From, msg.To, msg.RoomID)
}

func (h *Handlers) broadcastRoomStatus(room *models.Room) {
	status := room.GetRoomStatus()

	// In a real implementation, you would broadcast to all connected users
	// For now, we'll just log it
	log.Printf("Broadcasting room status for room %s: %d users", room.ID, status.UserCount)
}
