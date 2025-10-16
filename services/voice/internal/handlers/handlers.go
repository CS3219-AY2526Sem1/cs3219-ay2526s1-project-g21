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
		webrtcConfig: GetWebRTCConfig(),
	}
}

// placeholder to avoid depending on utils for this example
func GetWebRTCConfig() webrtc.Configuration { return webrtc.Configuration{} }

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
	config := struct {
		ICEServers []webrtc.ICEServer `json:"iceServers"`
	}{
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

	// Make sure connection is closed when this handler returns
	defer conn.Close()

	// The first message from client MUST be a join message with userId and username
	var joinMsg models.SignalingMessage
	if err := conn.ReadJSON(&joinMsg); err != nil {
		log.Printf("failed to read initial join message: %v", err)
		return
	}
	if joinMsg.Type != "join" {
		log.Printf("first message must be 'join'")
		return
	}

	// Extract user info from message data
	dataMap, ok := joinMsg.Data.(map[string]interface{})
	if !ok {
		// try via marshalling back to bytes to decode properly
		raw, _ := json.Marshal(joinMsg.Data)
		var tmp map[string]interface{}
		json.Unmarshal(raw, &tmp)
		dataMap = tmp
	}

	userID, _ := dataMap["userId"].(string)
	username, _ := dataMap["username"].(string)
	if userID == "" || username == "" {
		log.Printf("missing userId or username in join message")
		return
	}

	room := h.roomManager.GetOrCreateRoom(roomID)

	user := &models.User{ID: userID, Username: username, JoinedAt: time.Now()}
	room.AddUser(user)
	room.AddConn(userID, conn)

	// Broadcast updated room status
	h.broadcastRoomStatus(room)

	log.Printf("User %s joined room %s", username, room.ID)

	// Message loop
	for {
		var msg models.SignalingMessage
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("WebSocket read error for user %s: %v", userID, err)
			// remove user and connection
			room.RemoveUser(userID)
			room.RemoveConn(userID)
			h.roomManagerDeleteIfEmpty(room)
			h.broadcastRoomStatus(room)
			break
		}

		msg.Timestamp = time.Now()
		msg.RoomID = roomID

		switch msg.Type {
		case "leave":
			room.RemoveUser(msg.From)
			room.RemoveConn(msg.From)
			h.roomManagerDeleteIfEmpty(room)
			h.broadcastRoomStatus(room)
		case "offer", "answer", "ice-candidate":
			// forward to specific user
			h.forwardToUser(room, &msg)
		case "mute":
			if u, ok := room.GetUser(msg.From); ok {
				u.IsMuted = true
				h.broadcastRoomStatus(room)
			}
		case "unmute":
			if u, ok := room.GetUser(msg.From); ok {
				u.IsMuted = false
				h.broadcastRoomStatus(room)
			}
		default:
			log.Printf("unknown message type: %s", msg.Type)
		}
	}
}

func (h *Handlers) forwardToUser(room *models.Room, msg *models.SignalingMessage) {
	if msg.To == "" {
		log.Printf("no target specified for %s from %s", msg.Type, msg.From)
		return
	}
	if err := room.SendToUser(msg.To, msg); err != nil {
		log.Printf("failed to forward message to %s: %v", msg.To, err)
	}
}

func (h *Handlers) broadcastRoomStatus(room *models.Room) {
	status := room.GetRoomStatus()
	room.BroadcastJSON(status)
	log.Printf("Broadcasting room status for room %s: %d users", room.ID, status.UserCount)
}

// helper to delete room if empty
func (h *Handlers) roomManagerDeleteIfEmpty(room *models.Room) {
	if room.GetUserCount() == 0 {
		h.roomManager.DeleteRoom(room.ID)
		log.Printf("Deleted empty room %s", room.ID)
	}
}
