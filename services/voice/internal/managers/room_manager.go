package managers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
	"voice/internal/models"
	"voice/internal/utils"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RoomManager struct {
	rdb           *redis.Client
	roomStatusMap map[string]*models.RoomInfo // Match metadata cache (from Redis)
	rooms         map[string]*models.Room     // Active voice chat rooms (in-memory, per instance)
	mu            sync.RWMutex
	instanceID    string
	ctx           context.Context
	cancelPubSub  context.CancelFunc
}

func NewRoomManager(redisAddr string) *RoomManager {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	ctx, cancel := context.WithCancel(context.Background())

	rm := &RoomManager{
		rdb:           rdb,
		roomStatusMap: make(map[string]*models.RoomInfo),
		rooms:         make(map[string]*models.Room),
		instanceID:    uuid.New().String(), // Unique instance ID for multi-instance deployment
		ctx:           ctx,
		cancelPubSub:  cancel,
	}

	go rm.subscribeToPresenceEvents()

	log.Printf("[Voice RoomManager %s] Initialized", rm.instanceID)

	return rm
}

// GetInstanceID returns the instance ID for this room manager
func (rm *RoomManager) GetInstanceID() string {
	return rm.instanceID
}

// GetOrCreateRoom gets or creates an in-memory voice room
func (rm *RoomManager) GetOrCreateRoom(roomID string) *models.Room {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if room, exists := rm.rooms[roomID]; exists {
		return room
	}

	room := models.NewRoom(roomID)
	rm.rooms[roomID] = room
	log.Printf("[Voice RoomManager %s] Created new voice room: %s", rm.instanceID, roomID)
	return room
}

func (rm *RoomManager) DeleteRoom(roomID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.rooms, roomID)
	log.Printf("[Voice RoomManager %s] Deleted voice room: %s", rm.instanceID, roomID)
}

func (rm *RoomManager) PublishPresenceEvent(event *models.PresenceEvent) error {
	event.InstanceID = rm.instanceID
	event.Timestamp = time.Now()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal presence event: %w", err)
	}

	return rm.rdb.Publish(rm.ctx, "voice:presence", data).Err()
}

// subscribeToPresenceEvents listens for presence events from other service instances
func (rm *RoomManager) subscribeToPresenceEvents() {
	pubsub := rm.rdb.Subscribe(rm.ctx, "voice:presence")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Printf("Subscribed to voice presence events (instance: %s)", rm.instanceID)

	for {
		select {
		case <-rm.ctx.Done():
			log.Printf("Stopping presence event subscriber (instance: %s)", rm.instanceID)
			return
		case msg := <-ch:
			var event models.PresenceEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Printf("Failed to unmarshal presence event: %v", err)
				continue
			}

			// Ignore events from this instance
			if event.InstanceID == rm.instanceID {
				continue
			}

			rm.handlePresenceEvent(&event)
		}
	}
}

// handlePresenceEvent processes presence events from other service instances
func (rm *RoomManager) handlePresenceEvent(event *models.PresenceEvent) {
	log.Printf("Received presence event from instance %s: %s - %s in room %s",
		event.InstanceID, event.Type, event.UserID, event.RoomID)

	// Check if we have this room locally (user might have migrated instances)
	rm.mu.RLock()
	room, exists := rm.rooms[event.RoomID]
	rm.mu.RUnlock()

	if !exists {
		log.Printf("Room %s not found on instance %s, ignoring presence event",
			event.RoomID, rm.instanceID)
		return
	}

	// Update local room state based on event from other instance
	switch event.Type {
	case "user-joined":
		log.Printf("User %s joined room %s on instance %s",
			event.UserID, event.RoomID, event.InstanceID)

	case "user-left":
		if _, ok := room.GetUser(event.UserID); ok {
			log.Printf("User %s left room %s on instance %s, removing from local room",
				event.UserID, event.RoomID, event.InstanceID)
			room.RemoveUser(event.UserID)
			room.RemoveConn(event.UserID)

			status := room.GetRoomStatus()
			room.BroadcastJSON(status)
		}

	case "user-muted":
		if u, ok := room.GetUser(event.UserID); ok {
			log.Printf("User %s muted on instance %s, syncing local state",
				event.UserID, event.InstanceID)
			u.IsMuted = true

			status := room.GetRoomStatus()
			room.BroadcastJSON(status)
		}

	case "user-unmuted":
		if u, ok := room.GetUser(event.UserID); ok {
			log.Printf("User %s unmuted on instance %s, syncing local state",
				event.UserID, event.InstanceID)
			u.IsMuted = false

			status := room.GetRoomStatus()
			room.BroadcastJSON(status)
		}

	default:
		log.Printf("Unknown presence event type: %s", event.Type)
	}
}

func (rm *RoomManager) GetRoomStatus(matchId string) (*models.RoomInfo, error) {
	rm.mu.RLock()
	if roomInfo, exists := rm.roomStatusMap[matchId]; exists {
		copy := cloneRoomInfo(roomInfo)
		rm.mu.RUnlock()
		return copy, nil
	}
	rm.mu.RUnlock()

	roomInfo, err := rm.fetchRoomStatusFromRedis(matchId)
	if err != nil {
		return nil, err
	}

	rm.mu.Lock()
	rm.roomStatusMap[matchId] = roomInfo
	rm.mu.Unlock()

	return cloneRoomInfo(roomInfo), nil
}

func (rm *RoomManager) fetchRoomStatusFromRedis(matchId string) (*models.RoomInfo, error) {
	ctx := context.Background()
	roomKey := "room:" + matchId

	result := rm.rdb.HGetAll(ctx, roomKey)
	if result.Err() != nil {
		return nil, fmt.Errorf("failed to get room from Redis: %w", result.Err())
	}

	roomMap := result.Val()
	if len(roomMap) == 0 {
		return nil, fmt.Errorf("room not found")
	}

	roomInfo := &models.RoomInfo{
		MatchId:    roomMap["matchId"],
		User1:      roomMap["user1"],
		User2:      roomMap["user2"],
		Category:   roomMap["category"],
		Difficulty: roomMap["difficulty"],
		Status:     roomMap["status"],
		CreatedAt:  roomMap["createdAt"],
		Token1:     roomMap["token1"],
		Token2:     roomMap["token2"],
	}

	return roomInfo, nil
}

func cloneRoomInfo(src *models.RoomInfo) *models.RoomInfo {
	if src == nil {
		return nil
	}
	copy := *src
	return &copy
}

func (rm *RoomManager) ValidateRoomAccess(token string) (*models.RoomInfo, error) {
	claims, err := utils.ValidateRoomToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	roomInfo, err := rm.GetRoomStatus(claims.MatchId)
	if err != nil {
		return nil, fmt.Errorf("room not found: %w", err)
	}

	if claims.UserId != roomInfo.User1 && claims.UserId != roomInfo.User2 {
		return nil, fmt.Errorf("user not authorized for this room")
	}

	return roomInfo, nil
}

func (rm *RoomManager) Cleanup() {
	log.Printf("[Voice RoomManager %s] Starting cleanup", rm.instanceID)

	// Stop pub/sub subscriptions
	rm.cancelPubSub()

	// Close all active rooms and connections
	rm.mu.Lock()
	for roomID, room := range rm.rooms {
		log.Printf("[Voice RoomManager %s] Cleaning up room: %s", rm.instanceID, roomID)

		// Broadcast disconnection to all users
		room.BroadcastJSON(models.SignalingMessage{
			Type: "server-shutdown",
			Data: map[string]string{
				"message": "Server is shutting down. Please reconnect.",
			},
			Timestamp: time.Now(),
		})

		// Close all connections in the room
		for userID := range room.Connections {
			room.RemoveConn(userID)
		}
	}
	rm.rooms = make(map[string]*models.Room)
	rm.mu.Unlock()

	log.Printf("[Voice RoomManager %s] Cleanup completed", rm.instanceID)
}
