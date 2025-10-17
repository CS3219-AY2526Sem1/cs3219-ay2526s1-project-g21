package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"voice/internal/managers"
	"voice/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type Handlers struct {
	rdb         *redis.Client
	roomManager *managers.RoomManager
	upgrader    websocket.Upgrader
}

func NewHandlers(redisAddr string) *Handlers {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	return &Handlers{
		rdb:         rdb,
		roomManager: managers.NewRoomManager(redisAddr),
		upgrader:    websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
}

// Health check endpoint
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// GetWebRTCConfig returns STUN/TURN server configuration for WebRTC
func (h *Handlers) GetWebRTCConfig(w http.ResponseWriter, r *http.Request) {
	config := models.WebRTCConfig{
		IceServers: []models.IceServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
					"stun:stun1.l.google.com:19302",
					"stun:stun2.l.google.com:19302",
				},
			},
			// Add TURN servers here if needed for production
			// {
			// 	URLs:       []string{"turn:your-turn-server.com:3478"},
			// 	Username:   "username",
			// 	Credential: "password",
			// },
		},
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

	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "token is required", http.StatusUnauthorized)
		return
	}

	roomInfo, err := h.roomManager.ValidateRoomAccess(token)
	if err != nil {
		log.Printf("Token validation failed: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if roomInfo.MatchId != roomID {
		log.Printf("RoomID mismatch: token has %s, request has %s", roomInfo.MatchId, roomID)
		http.Error(w, "room mismatch", http.StatusForbidden)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

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

	dataMap, ok := joinMsg.Data.(map[string]interface{})
	if !ok {
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

	if userID != roomInfo.User1 && userID != roomInfo.User2 {
		log.Printf("UserID %s not authorized for room %s", userID, roomID)
		return
	}

	room := h.roomManager.GetOrCreateRoom(roomID)

	user := &models.User{ID: userID, Username: username, JoinedAt: time.Now()}
	room.AddUser(user)
	room.AddConn(userID, conn)

	h.roomManager.PublishPresenceEvent(&models.PresenceEvent{
		Type:     "user-joined",
		RoomID:   roomID,
		UserID:   userID,
		Username: username,
	})

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

			// Publish presence event
			h.roomManager.PublishPresenceEvent(&models.PresenceEvent{
				Type:   "user-left",
				RoomID: roomID,
				UserID: userID,
			})

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

			h.roomManager.PublishPresenceEvent(&models.PresenceEvent{
				Type:   "user-left",
				RoomID: roomID,
				UserID: msg.From,
			})

			h.roomManagerDeleteIfEmpty(room)
			h.broadcastRoomStatus(room)
		case "offer", "answer", "ice-candidate":
			// forward to specific user
			h.forwardToUser(room, &msg)
		case "mute":
			if u, ok := room.GetUser(msg.From); ok {
				u.IsMuted = true

				h.roomManager.PublishPresenceEvent(&models.PresenceEvent{
					Type:    "user-muted",
					RoomID:  roomID,
					UserID:  msg.From,
					IsMuted: true,
				})

				h.broadcastRoomStatus(room)
			}
		case "unmute":
			if u, ok := room.GetUser(msg.From); ok {
				u.IsMuted = false

				h.roomManager.PublishPresenceEvent(&models.PresenceEvent{
					Type:    "user-unmuted",
					RoomID:  roomID,
					UserID:  msg.From,
					IsMuted: false,
				})

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
