package match_management

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"match/internal/elo"
	"match/internal/models"
	"match/internal/utils"
)

const (
	MatchHandshakeTimeout = 15
	STAGE1_TIMEOUT        = 100
	STAGE2_TIMEOUT        = 200
	STAGE3_TIMEOUT        = 300
	RoomExpiration        = 2 * time.Hour
)

type MatchManager struct {
	ctx       context.Context
	rdb       *redis.Client
	pubClient *redis.Client // For publishing messages
	subClient *redis.Client // For subscribing (needs separate connection)
	upgrader  websocket.Upgrader

	// Only store LOCAL WebSocket connections (not shared between instances)
	connections map[string]*websocket.Conn
	mu          sync.Mutex

	jwtSecret []byte

	// Instance ID for debugging
	instanceID string

	// Elo rating manager
	eloManager *elo.EloManager
}

func NewMatchManager(secret []byte, rdb *redis.Client) *MatchManager {
	// Create separate Redis clients for pub/sub
	subClient := redis.NewClient(&redis.Options{
		Addr: rdb.Options().Addr,
		DB:   rdb.Options().DB,
	})

	// Test connection
	if err := subClient.Ping(context.Background()).Err(); err != nil {
		log.Printf("WARNING: subClient ping failed: %v", err)
	} else {
		log.Printf("subClient connected successfully")
	}

	mm := &MatchManager{
		ctx:       context.Background(),
		rdb:       rdb,
		pubClient: rdb,
		subClient: subClient,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		connections: make(map[string]*websocket.Conn),
		jwtSecret:   secret,
		instanceID:  uuid.New().String()[:8], // Short ID for logging
		eloManager:  elo.NewEloManager(rdb),
	}

	// Start background subscribers
	go mm.subscribeToUserMessages()
	go mm.subscribeToRedis()

	log.Printf("Match Manager instance %s started", mm.instanceID)

	return mm
}

// --- Redis Pub/Sub for WebSocket Messages ---
// This allows any instance to send messages to users connected to any other instance
func (mm *MatchManager) subscribeToUserMessages() {
	pubsub := mm.subClient.PSubscribe(mm.ctx, "user:*:message")
	defer pubsub.Close()

	ch := pubsub.Channel()
	log.Printf("[Instance %s] Subscribed to user message channels", mm.instanceID)

	for msg := range ch {
		// Extract userId from channel name: "user:123:message"
		if len(msg.Channel) < 13 { // "user:X:message" minimum length
			continue
		}
		userId := msg.Channel[5 : len(msg.Channel)-8] // Remove "user:" and ":message"
		log.Printf("[Instance %s] Received message for user %s", mm.instanceID, userId)

		mm.mu.Lock()
		conn, ok := mm.connections[userId]
		mm.mu.Unlock()

		if ok {
			// This user is connected to THIS instance, forward the message
			var data interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &data); err != nil {
				log.Printf("[Instance %s] Failed to parse message for user %s: %v", mm.instanceID, userId, err)
				continue
			}

			if err := conn.WriteJSON(data); err != nil {
				log.Printf("[Instance %s] Error sending to user %s: %v", mm.instanceID, userId, err)
				mm.mu.Lock()
				delete(mm.connections, userId)
				mm.mu.Unlock()
				conn.Close()
			} else {
				log.Printf("[Instance %s] Delivered message to user %s", mm.instanceID, userId)
			}
		}
		// If user not connected to this instance, ignore (another instance will handle it)
	}
}

// --- Redis Subscriber for Events ---
func (mm *MatchManager) subscribeToRedis() {
	subscriber := mm.subClient.Subscribe(mm.ctx, "matches", "session_ended")
	ch := subscriber.Channel()

	log.Printf("[Instance %s] Subscribed to matches and session_ended channels", mm.instanceID)

	for msg := range ch {
		if msg.Channel == "session_ended" {
			mm.handleSessionEndedEvent(msg.Payload)
		} else {
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Println("Failed to parse event:", err)
				continue
			}
			log.Printf("[Instance %s] Redis event received: %v", mm.instanceID, event)
		}
	}
}

// Handle session_ended events to clean up match service state
func (mm *MatchManager) handleSessionEndedEvent(payload string) {
	var event struct {
		MatchID string `json:"matchId"`
		User1   string `json:"user1"`
		User2   string `json:"user2"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("[Instance %s] Failed to parse session_ended event: %v", mm.instanceID, err)
		return
	}

	// Clean up Redis state (shared across all instances)
	mm.rdb.Del(mm.ctx, fmt.Sprintf("user_room:%s", event.User1))
	mm.rdb.Del(mm.ctx, fmt.Sprintf("user_room:%s", event.User2))
	mm.rdb.Del(mm.ctx, fmt.Sprintf("room:%s", event.MatchID))

	log.Printf("[Instance %s] Cleaned up match service state for ended session %s", mm.instanceID, event.MatchID)
}

// --- Matchmaking Loop ---
func (mm *MatchManager) StartMatchmakingLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	log.Printf("[Instance %s] Started matchmaking loop", mm.instanceID)

	for range ticker.C {
		keys, _ := mm.rdb.Keys(mm.ctx, "user:*").Result()
		for _, key := range keys {
			user, _ := mm.rdb.HGetAll(mm.ctx, key).Result()
			if len(user) == 0 {
				continue
			}

			userId := key[5:]
			category := user["category"]
			difficulty := user["difficulty"]
			stage, _ := strconv.Atoi(user["stage"])
			joinedAt, _ := strconv.ParseFloat(user["joined_at"], 64)
			elapsed := time.Now().Unix() - int64(joinedAt)

			switch stage {
			case 1:
				if elapsed > STAGE1_TIMEOUT {
					mm.rdb.HSet(mm.ctx, key, "stage", 2)
					mm.tryMatchStage(category, difficulty, 2)
				}
			case 2:
				if elapsed > STAGE2_TIMEOUT {
					mm.rdb.HSet(mm.ctx, key, "stage", 3)
					mm.tryMatchStage(category, difficulty, 3)
				}
			case 3:
				if elapsed > STAGE3_TIMEOUT {
					mm.removeUser(userId, category, difficulty)
					mm.sendToUser(userId, map[string]interface{}{
						"type":    "timeout",
						"message": "Matchmaking timed out",
					})
				}
			}
		}
	}
}

// --- Pending Match Expiration Loop ---
// Now checks Redis instead of local memory
func (mm *MatchManager) StartPendingMatchExpirationLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	log.Printf("[Instance %s] Started pending match expiration loop", mm.instanceID)

	for range ticker.C {
		// Find all pending matches in Redis
		pendingKeys, _ := mm.rdb.Keys(mm.ctx, "pending_match:*").Result()

		for _, pendingKey := range pendingKeys {
			pendingJSON, err := mm.rdb.Get(mm.ctx, pendingKey).Result()
			if err != nil {
				continue // Already expired or deleted
			}

			var pending models.PendingMatch
			if err := json.Unmarshal([]byte(pendingJSON), &pending); err != nil {
				log.Printf("[Instance %s] Failed to parse pending match: %v", mm.instanceID, err)
				continue
			}

			// Check if expired
			if time.Now().After(pending.ExpiresAt) {
				mm.handleExpiredMatch(&pending)
			}
		}
	}
}

// Handle expired pending match
func (mm *MatchManager) handleExpiredMatch(pending *models.PendingMatch) {
	matchID := pending.MatchId
	log.Printf("[Instance %s] Match %s expired - checking handshakes", mm.instanceID, matchID)

	// Check handshake status
	h1, err1 := mm.rdb.Get(mm.ctx, fmt.Sprintf("handshake:%s:%s", matchID, pending.User1)).Result()
	h2, err2 := mm.rdb.Get(mm.ctx, fmt.Sprintf("handshake:%s:%s", matchID, pending.User2)).Result()

	user1Accepted := err1 == nil && h1 == "accepted"
	user2Accepted := err2 == nil && h2 == "accepted"

	// Re-queue users who accepted
	if user1Accepted {
		mm.requeueUser(pending.User1, pending.User1Cat, pending.User1Diff)
		mm.sendToUser(pending.User1, map[string]interface{}{
			"type":    "requeued",
			"message": "Other user did not accept in time. You have been re-queued.",
		})
	} else {
		mm.removeUser(pending.User1, pending.User1Cat, pending.User1Diff)
		mm.sendToUser(pending.User1, map[string]interface{}{
			"type":    "timeout",
			"message": "Match expired - you did not accept in time",
		})
	}

	if user2Accepted {
		mm.requeueUser(pending.User2, pending.User2Cat, pending.User2Diff)
		mm.sendToUser(pending.User2, map[string]interface{}{
			"type":    "requeued",
			"message": "Other user did not accept in time. You have been re-queued.",
		})
	} else {
		mm.removeUser(pending.User2, pending.User2Cat, pending.User2Diff)
		mm.sendToUser(pending.User2, map[string]interface{}{
			"type":    "timeout",
			"message": "Match expired - you did not accept in time",
		})
	}

	// Clean up Redis
	mm.rdb.Del(mm.ctx, fmt.Sprintf("pending_match:%s", matchID))
	mm.rdb.Del(mm.ctx, fmt.Sprintf("handshake:%s:%s", matchID, pending.User1))
	mm.rdb.Del(mm.ctx, fmt.Sprintf("handshake:%s:%s", matchID, pending.User2))
}

// Re-queue a user (preserving original timestamp)
func (mm *MatchManager) requeueUser(userId, category, difficulty string) {
	userKey := fmt.Sprintf("user:%s", userId)

	// Get original timestamp if exists, otherwise use current time
	var originalTime float64
	userData, _ := mm.rdb.HGetAll(mm.ctx, userKey).Result()
	if joinedAt, ok := userData["joined_at"]; ok {
		originalTime, _ = strconv.ParseFloat(joinedAt, 64)
	} else {
		originalTime = float64(time.Now().Unix())
	}

	// Re-add to queue
	mm.rdb.HSet(mm.ctx, userKey, map[string]interface{}{
		"category":   category,
		"difficulty": difficulty,
		"joined_at":  originalTime,
		"stage":      1,
	})
	mm.rdb.ZAdd(mm.ctx, fmt.Sprintf("queue:%s:%s", category, difficulty), redis.Z{Score: originalTime, Member: userId})
	mm.rdb.ZAdd(mm.ctx, fmt.Sprintf("queue:%s", category), redis.Z{Score: originalTime, Member: userId})
	mm.rdb.ZAdd(mm.ctx, "queue:all", redis.Z{Score: originalTime, Member: userId})

	log.Printf("[Instance %s] Re-queued user %s", mm.instanceID, userId)
}

// --- Try Match at Stage ---
func (mm *MatchManager) tryMatchStage(category, difficulty string, stage int) {
	var queueKeys []string
	switch stage {
	case 1:
		queueKeys = []string{fmt.Sprintf("queue:%s:%s", category, difficulty)}
	case 2:
		queueKeys = []string{
			fmt.Sprintf("queue:%s:%s", category, difficulty),
			fmt.Sprintf("queue:%s", category),
		}
	case 3:
		queueKeys = []string{
			fmt.Sprintf("queue:%s:%s", category, difficulty),
			fmt.Sprintf("queue:%s", category),
			"queue:all",
		}
	default:
		return
	}

	for _, queueKey := range queueKeys {
		// Get more users to check for Elo compatibility
		users, _ := mm.rdb.ZRange(mm.ctx, queueKey, 0, 9).Result() // Get up to 10 users
		if len(users) < 2 {
			continue
		}

		// 1st pass: strict, avoid re-matches
		if mm.tryMatch(users, stage, false) {
			return
		}

		// 2nd pass: allow re-matches
		if mm.tryMatch(users, stage, true) {
			return
		}
	}
}

// tryMatch attempts to find a match in the given user list.
// If allowRecentMatch = false, it skips pairs with recent matches.
func (mm *MatchManager) tryMatch(users []string, stage int, allowRecentMatch bool) bool {
	for i := 0; i < len(users)-1; i++ {
		u1 := users[i]
		user1Data, _ := mm.rdb.HGetAll(mm.ctx, fmt.Sprintf("user:%s", u1)).Result()
		user1Elo, _ := mm.eloManager.GetUserElo(u1)

		for j := i + 1; j < len(users); j++ {
			u2 := users[j]
			user2Data, _ := mm.rdb.HGetAll(mm.ctx, fmt.Sprintf("user:%s", u2)).Result()
			user2Elo, _ := mm.eloManager.GetUserElo(u2)

			// Elo compatibility check
			if !elo.CheckEloCompatibility(user1Elo.EloRating, user2Elo.EloRating, stage) {
				continue
			}

			// Respect recent match rule if required
			if !allowRecentMatch && mm.hasRecentMatch(u1, u2) {
				continue
			}

			// Log re-match if allowed
			if allowRecentMatch && mm.hasRecentMatch(u1, u2) {
				log.Printf("[Instance %s] Allowing re-match (fallback): %s and %s",
					mm.instanceID, u1, u2)
			}

			log.Printf("[Instance %s] Found compatible match at stage %d: %s (Elo: %.0f) and %s (Elo: %.0f)",
				mm.instanceID, stage, u1, user1Elo.EloRating, u2, user2Elo.EloRating)

			mm.createPendingMatch(
				u1, u2,
				user1Data["category"], user1Data["difficulty"],
				user2Data["category"], user2Data["difficulty"],
				stage,
			)
			return true
		}
	}
	return false
}

// --- Create Pending Match (now stores in Redis) ---
func (mm *MatchManager) createPendingMatch(u1, u2, cat1, diff1, cat2, diff2 string, stage int) {
	log.Printf("[Instance %s] Creating pending match between %s (%s/%s) and %s (%s/%s) at stage %d",
		mm.instanceID, u1, cat1, diff1, u2, cat2, diff2, stage)

	var finalCat, finalDiff string

	switch stage {
	case 1:
		finalCat = cat1
		finalDiff = diff1
	case 2:
		finalCat = cat1
		finalDiff = utils.GetAverageDifficulty(diff1, diff2)
	case 3:
		finalCat = utils.GetRandomCategory(cat1, cat2)
		finalDiff = utils.GetAverageDifficulty(diff1, diff2)
	}

	// Remove users from their respective queues
	mm.rdb.ZRem(mm.ctx, fmt.Sprintf("queue:%s:%s", cat1, diff1), u1)
	mm.rdb.ZRem(mm.ctx, fmt.Sprintf("queue:%s", cat1), u1)
	mm.rdb.ZRem(mm.ctx, "queue:all", u1)

	mm.rdb.ZRem(mm.ctx, fmt.Sprintf("queue:%s:%s", cat2, diff2), u2)
	mm.rdb.ZRem(mm.ctx, fmt.Sprintf("queue:%s", cat2), u2)
	mm.rdb.ZRem(mm.ctx, "queue:all", u2)

	matchID := uuid.New().String()
	token1, _ := utils.GenerateRoomToken(matchID, u1, mm.jwtSecret)
	token2, _ := utils.GenerateRoomToken(matchID, u2, mm.jwtSecret)

	pending := &models.PendingMatch{
		MatchId:    matchID,
		User1:      u1,
		User2:      u2,
		Category:   finalCat,
		Difficulty: finalDiff,
		User1Cat:   cat1,
		User1Diff:  diff1,
		User2Cat:   cat2,
		User2Diff:  diff2,
		Token1:     token1,
		Token2:     token2,
		Handshakes: make(map[string]bool),
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(MatchHandshakeTimeout * time.Second),
	}

	// Store in Redis with expiration (shared across all instances)
	pendingJSON, _ := json.Marshal(pending)
	mm.rdb.Set(mm.ctx, fmt.Sprintf("pending_match:%s", matchID), pendingJSON, (MatchHandshakeTimeout+5)*time.Second)

	// Create handshake tracking keys
	mm.rdb.Set(mm.ctx, fmt.Sprintf("handshake:%s:%s", matchID, u1), "pending", (MatchHandshakeTimeout+5)*time.Second)
	mm.rdb.Set(mm.ctx, fmt.Sprintf("handshake:%s:%s", matchID, u2), "pending", (MatchHandshakeTimeout+5)*time.Second)

	// Notify both users (via Redis pub/sub, works across instances)
	mm.sendToUser(u1, map[string]interface{}{
		"type":       "match_pending",
		"matchId":    matchID,
		"category":   finalCat,
		"difficulty": finalDiff,
		"expiresIn":  MatchHandshakeTimeout,
	})

	mm.sendToUser(u2, map[string]interface{}{
		"type":       "match_pending",
		"matchId":    matchID,
		"category":   finalCat,
		"difficulty": finalDiff,
		"expiresIn":  MatchHandshakeTimeout,
	})
}

// --- Handle Match Accept ---
func (mm *MatchManager) HandleMatchAccept(matchID, userId string) error {
	log.Printf("[Instance %s] User %s accepted match %s", mm.instanceID, userId, matchID)

	// Mark handshake as accepted in Redis
	key := fmt.Sprintf("handshake:%s:%s", matchID, userId)
	err := mm.rdb.Set(mm.ctx, key, "accepted", (MatchHandshakeTimeout+5)*time.Second).Err()
	if err != nil {
		return fmt.Errorf("failed to mark handshake: %w", err)
	}

	// Get pending match from Redis
	pendingKey := fmt.Sprintf("pending_match:%s", matchID)
	pendingJSON, err := mm.rdb.Get(mm.ctx, pendingKey).Result()
	if err != nil {
		return fmt.Errorf("pending match not found: %w", err)
	}

	var pending models.PendingMatch
	if err := json.Unmarshal([]byte(pendingJSON), &pending); err != nil {
		return fmt.Errorf("failed to parse pending match: %w", err)
	}

	// Check if both users accepted
	h1, err1 := mm.rdb.Get(mm.ctx, fmt.Sprintf("handshake:%s:%s", matchID, pending.User1)).Result()
	h2, err2 := mm.rdb.Get(mm.ctx, fmt.Sprintf("handshake:%s:%s", matchID, pending.User2)).Result()

	if err1 == nil && h1 == "accepted" && err2 == nil && h2 == "accepted" {
		// Both accepted! Finalize match
		log.Printf("[Instance %s] Both users accepted match %s - finalizing", mm.instanceID, matchID)
		mm.finalizeMatch(&pending)

		// Clean up Redis keys
		mm.rdb.Del(mm.ctx, pendingKey)
		mm.rdb.Del(mm.ctx, fmt.Sprintf("handshake:%s:%s", matchID, pending.User1))
		mm.rdb.Del(mm.ctx, fmt.Sprintf("handshake:%s:%s", matchID, pending.User2))
	}

	return nil
}

// --- Finalize Match ---
func (mm *MatchManager) finalizeMatch(pending *models.PendingMatch) {
	// Store room info in Redis (shared across instances)
	mm.rdb.Set(mm.ctx, fmt.Sprintf("user_room:%s", pending.User1), pending.MatchId, RoomExpiration)
	mm.rdb.Set(mm.ctx, fmt.Sprintf("user_room:%s", pending.User2), pending.MatchId, RoomExpiration)

	roomInfo := models.RoomInfo{
		MatchId:    pending.MatchId,
		User1:      pending.User1,
		User2:      pending.User2,
		Category:   pending.Category,
		Difficulty: pending.Difficulty,
		Status:     "active",
		Token1:     pending.Token1,
		Token2:     pending.Token2,
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	roomJSON, _ := json.Marshal(roomInfo)
	mm.rdb.Set(mm.ctx, fmt.Sprintf("room:%s", pending.MatchId), roomJSON, RoomExpiration)

	// Send tokens to both users
	mm.sendToUser(pending.User1, map[string]interface{}{
		"type":       "match_confirmed",
		"matchId":    pending.MatchId,
		"token":      pending.Token1,
		"category":   pending.Category,
		"difficulty": pending.Difficulty,
	})

	mm.sendToUser(pending.User2, map[string]interface{}{
		"type":       "match_confirmed",
		"matchId":    pending.MatchId,
		"token":      pending.Token2,
		"category":   pending.Category,
		"difficulty": pending.Difficulty,
	})

	// Publish match event
	matchEvent := map[string]interface{}{
		"type":       "match_created",
		"matchId":    pending.MatchId,
		"user1":      pending.User1,
		"user2":      pending.User2,
		"category":   pending.Category,
		"difficulty": pending.Difficulty,
	}
	eventJSON, _ := json.Marshal(matchEvent)
	mm.pubClient.Publish(mm.ctx, "matches", eventJSON)

	log.Printf("[Instance %s] Match %s finalized successfully", mm.instanceID, pending.MatchId)

	// Record match history to prevent re-matches
	mm.recordMatch(pending.User1, pending.User2)
}

// --- Match History Management ---

// hasRecentMatch checks if two users have matched within the last 24 hours
func (mm *MatchManager) hasRecentMatch(user1, user2 string) bool {
	// Check if user2 is in user1's recent partners
	key1 := fmt.Sprintf("user_history:%s:partners", user1)
	isMember, err := mm.rdb.SIsMember(mm.ctx, key1, user2).Result()
	if err == nil && isMember {
		return true
	}

	// Check if user1 is in user2's recent partners
	key2 := fmt.Sprintf("user_history:%s:partners", user2)
	isMember, err = mm.rdb.SIsMember(mm.ctx, key2, user1).Result()
	if err == nil && isMember {
		return true
	}

	return false
}

// recordMatch records a match between two users with 24-hour expiration
func (mm *MatchManager) recordMatch(user1, user2 string) {
	// Add each user to the other's recent partners set
	key1 := fmt.Sprintf("user_history:%s:partners", user1)
	key2 := fmt.Sprintf("user_history:%s:partners", user2)

	// Add user2 to user1's partners
	mm.rdb.SAdd(mm.ctx, key1, user2)
	mm.rdb.Expire(mm.ctx, key1, 24*time.Hour)

	// Add user1 to user2's partners
	mm.rdb.SAdd(mm.ctx, key2, user1)
	mm.rdb.Expire(mm.ctx, key2, 24*time.Hour)

	log.Printf("[Instance %s] Recorded match history between %s and %s", mm.instanceID, user1, user2)
}

// --- Remove User ---
func (mm *MatchManager) removeUser(userId, category, difficulty string) {
	log.Printf("[Instance %s] Removing user %s from queues", mm.instanceID, userId)
	mm.rdb.Del(mm.ctx, fmt.Sprintf("user:%s", userId))
	mm.rdb.ZRem(mm.ctx, fmt.Sprintf("queue:%s:%s", category, difficulty), userId)
	mm.rdb.ZRem(mm.ctx, fmt.Sprintf("queue:%s", category), userId)
	mm.rdb.ZRem(mm.ctx, "queue:all", userId)
}

// --- Send To User (via Redis Pub/Sub) ---
// This publishes to a Redis channel that ALL instances subscribe to
// The instance with the actual WebSocket connection will deliver it
func (mm *MatchManager) sendToUser(userId string, data interface{}) {
	payload, err := json.Marshal(data)
	if err != nil {
		log.Printf("[Instance %s] Error marshaling message for user %s: %v", mm.instanceID, userId, err)
		return
	}

	// Publish to Redis channel for this user
	channel := fmt.Sprintf("user:%s:message", userId)
	err = mm.pubClient.Publish(mm.ctx, channel, payload).Err()
	if err != nil {
		log.Printf("[Instance %s] Error publishing message for user %s: %v", mm.instanceID, userId, err)
	} else {
		log.Printf("[Instance %s] Published message to channel %s", mm.instanceID, channel)
	}
}

// --- Register WebSocket Connection (local to this instance) ---
func (mm *MatchManager) RegisterConnection(userId string, conn *websocket.Conn) {
	mm.mu.Lock()
	mm.connections[userId] = conn
	mm.mu.Unlock()
	log.Printf("[Instance %s] Registered WebSocket connection for user %s", mm.instanceID, userId)
}

// --- Unregister WebSocket Connection ---
func (mm *MatchManager) UnregisterConnection(userId string) {
	mm.mu.Lock()
	delete(mm.connections, userId)
	mm.mu.Unlock()
	log.Printf("[Instance %s] Unregistered WebSocket connection for user %s", mm.instanceID, userId)
}

// --- Get Room for User (from Redis) ---
func (mm *MatchManager) GetRoomForUser(userId string) (string, error) {
	return mm.rdb.Get(mm.ctx, fmt.Sprintf("user_room:%s", userId)).Result()
}

// --- Get Room Info (from Redis) ---
func (mm *MatchManager) GetRoomInfo(matchID string) (*models.RoomInfo, error) {
	roomJSON, err := mm.rdb.Get(mm.ctx, fmt.Sprintf("room:%s", matchID)).Result()
	if err != nil {
		return nil, err
	}

	var roomInfo models.RoomInfo
	if err := json.Unmarshal([]byte(roomJSON), &roomInfo); err != nil {
		return nil, err
	}

	return &roomInfo, nil
}
