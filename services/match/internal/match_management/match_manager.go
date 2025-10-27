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

	"match/internal/models"
	"match/internal/utils"
)

const (
	MatchHandshakeTimeout = 15
	STAGE1_TIMEOUT        = 100
	STAGE2_TIMEOUT        = 200
	STAGE3_TIMEOUT        = 300
)

type MatchManager struct {
	ctx         context.Context
	rdb         *redis.Client
	upgrader    websocket.Upgrader
	connections map[string]*websocket.Conn
	mu          sync.Mutex
	jwtSecret   []byte

	// Maps for room tracking
	userToRoom map[string]string
	roomInfo   map[string]*models.RoomInfo
	roomMu     sync.RWMutex

	// Pending matches awaiting handshake
	pendingMatches map[string]*models.PendingMatch
	pendingMu      sync.Mutex
}

func NewMatchManager(secret []byte, rdb *redis.Client) *MatchManager {
	return &MatchManager{
		ctx: context.Background(),
		rdb: rdb,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		connections: make(map[string]*websocket.Conn),
		jwtSecret:   secret,

		userToRoom: make(map[string]string),
		roomInfo:   make(map[string]*models.RoomInfo),

		pendingMatches: make(map[string]*models.PendingMatch),
	}
}

// --- Redis Subscriber ---
func (matchManager *MatchManager) SubscribeToRedis() {
	subscriber := matchManager.rdb.Subscribe(matchManager.ctx, "matches", "session_ended")
	ch := subscriber.Channel()

	for msg := range ch {
		// Handle different event types based on channel
		if msg.Channel == "session_ended" {
			matchManager.handleSessionEndedEvent(msg.Payload)
		} else {
			var event map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				log.Println("Failed to parse event:", err)
				continue
			}
			log.Printf("Redis event received: %v", event)
		}
	}
}

// Handle session_ended events to clean up match service state
func (matchManager *MatchManager) handleSessionEndedEvent(payload string) {
	var event struct {
		MatchID string `json:"matchId"`
		User1   string `json:"user1"`
		User2   string `json:"user2"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("Failed to parse session_ended event: %v", err)
		return
	}

	matchManager.roomMu.Lock()
	defer matchManager.roomMu.Unlock()

	// Remove both users from tracking
	delete(matchManager.userToRoom, event.User1)
	delete(matchManager.userToRoom, event.User2)

	// Remove room info
	delete(matchManager.roomInfo, event.MatchID)

	log.Printf("Cleaned up match service state for ended session %s", event.MatchID)
}

func (matchManager *MatchManager) StartMatchmakingLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		keys, _ := matchManager.rdb.Keys(matchManager.ctx, "user:*").Result()
		for _, key := range keys {
			user, _ := matchManager.rdb.HGetAll(matchManager.ctx, key).Result()
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
					matchManager.rdb.HSet(matchManager.ctx, key, "stage", 2)
					matchManager.tryMatchStage(category, difficulty, 2)
				}
			case 2:
				if elapsed > STAGE2_TIMEOUT {
					matchManager.rdb.HSet(matchManager.ctx, key, "stage", 3)
					matchManager.tryMatchStage(category, difficulty, 3)
				}
			case 3:
				if elapsed > STAGE3_TIMEOUT {
					matchManager.removeUser(userId, category, difficulty)
					matchManager.sendToUser(userId, map[string]interface{}{
						"type":    "timeout",
						"message": "Matchmaking timed out",
					})
				}
			}
		}
	}
}

// --- Pending Match Expiration Loop ---
func (matchManager *MatchManager) StartPendingMatchExpirationLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()

		matchManager.pendingMu.Lock()
		for matchId, pending := range matchManager.pendingMatches {
			if now.After(pending.ExpiresAt) {
				log.Printf("Match %s expired - not all users accepted in time", matchId)

				// Re-queue users who accepted
				for userId, accepted := range pending.Handshakes {
					if accepted {
						var cat, diff string
						var originalTime float64

						if userId == pending.User1 {
							cat = pending.User1Cat
							diff = pending.User1Diff
						} else {
							cat = pending.User2Cat
							diff = pending.User2Diff
						}

						userKey := fmt.Sprintf("user:%s", userId)
						userData, _ := matchManager.rdb.HGetAll(matchManager.ctx, userKey).Result()
						if joinedAt, ok := userData["joined_at"]; ok {
							originalTime, _ = strconv.ParseFloat(joinedAt, 64)
						} else {
							originalTime = float64(time.Now().Unix())
						}

						// Re-add to queue
						matchManager.rdb.HSet(matchManager.ctx, userKey, map[string]interface{}{
							"category":   cat,
							"difficulty": diff,
							"joined_at":  originalTime,
							"stage":      1,
						})
						matchManager.rdb.ZAdd(matchManager.ctx, fmt.Sprintf("queue:%s:%s", cat, diff), redis.Z{Score: originalTime, Member: userId})
						matchManager.rdb.ZAdd(matchManager.ctx, fmt.Sprintf("queue:%s", cat), redis.Z{Score: originalTime, Member: userId})
						matchManager.rdb.ZAdd(matchManager.ctx, "queue:all", redis.Z{Score: originalTime, Member: userId})

						matchManager.sendToUser(userId, map[string]interface{}{
							"type":    "requeued",
							"message": "Other user did not accept in time. You have been re-queued.",
						})
					}
				}

				// Remove users who didn't accept
				if !pending.Handshakes[pending.User1] {
					matchManager.removeUser(pending.User1, pending.User1Cat, pending.User1Diff)
					matchManager.sendToUser(pending.User1, map[string]interface{}{
						"type":    "timeout",
						"message": "Match expired - you did not accept in time",
					})
				}
				if !pending.Handshakes[pending.User2] {
					matchManager.removeUser(pending.User2, pending.User2Cat, pending.User2Diff)
					matchManager.sendToUser(pending.User2, map[string]interface{}{
						"type":    "timeout",
						"message": "Match expired - you did not accept in time",
					})
				}

				delete(matchManager.pendingMatches, matchId)
			}
		}
		matchManager.pendingMu.Unlock()
	}
}

// --- Try Match at Stage ---
func (matchManager *MatchManager) tryMatchStage(category, difficulty string, stage int) {
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
		users, _ := matchManager.rdb.ZRange(matchManager.ctx, queueKey, 0, 1).Result()
		if len(users) < 2 {
			continue
		}

		u1, u2 := users[0], users[1]

		// Get their original preferences
		user1Data, _ := matchManager.rdb.HGetAll(matchManager.ctx, fmt.Sprintf("user:%s", u1)).Result()
		user2Data, _ := matchManager.rdb.HGetAll(matchManager.ctx, fmt.Sprintf("user:%s", u2)).Result()

		matchManager.createPendingMatch(u1, u2,
			user1Data["category"], user1Data["difficulty"],
			user2Data["category"], user2Data["difficulty"], stage)
		return
	}
}

// --- Create Pending Match ---
func (matchManager *MatchManager) createPendingMatch(u1, u2, cat1, diff1, cat2, diff2 string, stage int) {
	log.Printf("Creating pending match between %s (%s/%s) and %s (%s/%s) at stage %d",
		u1, cat1, diff1, u2, cat2, diff2, stage)

	var finalCat, finalDiff string

	switch stage {
	case 1:
		// Strict match - same category and difficulty
		finalCat = cat1
		finalDiff = diff1
	case 2:
		// Category match - average difficulty
		finalCat = cat1
		finalDiff = utils.GetAverageDifficulty(diff1, diff2)
	case 3:
		// Any match - random category, average difficulty
		finalCat = utils.GetRandomCategory(cat1, cat2)
		finalDiff = utils.GetAverageDifficulty(diff1, diff2)
	}

	// Remove users from their respective queues
	matchManager.rdb.ZRem(matchManager.ctx, fmt.Sprintf("queue:%s:%s", cat1, diff1), u1)
	matchManager.rdb.ZRem(matchManager.ctx, fmt.Sprintf("queue:%s", cat1), u1)
	matchManager.rdb.ZRem(matchManager.ctx, "queue:all", u1)

	matchManager.rdb.ZRem(matchManager.ctx, fmt.Sprintf("queue:%s:%s", cat2, diff2), u2)
	matchManager.rdb.ZRem(matchManager.ctx, fmt.Sprintf("queue:%s", cat2), u2)
	matchManager.rdb.ZRem(matchManager.ctx, "queue:all", u2)

	matchID := uuid.New().String()
	token1, _ := utils.GenerateRoomToken(matchID, u1, matchManager.jwtSecret)
	token2, _ := utils.GenerateRoomToken(matchID, u2, matchManager.jwtSecret)

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

	matchManager.pendingMu.Lock()
	matchManager.pendingMatches[matchID] = pending
	matchManager.pendingMu.Unlock()

	// Notify both users about pending match
	matchManager.sendToUser(u1, map[string]interface{}{
		"type":       "match_pending",
		"matchId":    matchID,
		"category":   finalCat,
		"difficulty": finalDiff,
		"expiresIn":  MatchHandshakeTimeout,
	})

	matchManager.sendToUser(u2, map[string]interface{}{
		"type":       "match_pending",
		"matchId":    matchID,
		"category":   finalCat,
		"difficulty": finalDiff,
		"expiresIn":  MatchHandshakeTimeout,
	})
}

// --- Remove User ---
func (matchManager *MatchManager) removeUser(userId, category, difficulty string) {
	log.Printf("Removing user %s from queues", userId)
	matchManager.rdb.Del(matchManager.ctx, fmt.Sprintf("user:%s", userId))
	matchManager.rdb.ZRem(matchManager.ctx, fmt.Sprintf("queue:%s:%s", category, difficulty), userId)
	matchManager.rdb.ZRem(matchManager.ctx, fmt.Sprintf("queue:%s", category), userId)
	matchManager.rdb.ZRem(matchManager.ctx, "queue:all", userId)
}

func (matchManager *MatchManager) sendToUser(userId string, data interface{}) {
	matchManager.mu.Lock()
	conn, ok := matchManager.connections[userId]
	matchManager.mu.Unlock()

	if ok {
		if err := conn.WriteJSON(data); err != nil {
			log.Printf("Error sending to user %s: %v", userId, err)
			matchManager.mu.Lock()
			delete(matchManager.connections, userId)
			matchManager.mu.Unlock()
			conn.Close()
		}
	}
}
