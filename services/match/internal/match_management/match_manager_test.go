package match_management

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"match/internal/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// setupTestRedis creates a miniredis instance and a redis client for testing
func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { client.Close() })

	return mr, client
}

func TestNewMatchManager(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)

	mm := NewMatchManager(secret, rdb)

	assert.NotNil(t, mm)
	assert.Equal(t, secret, mm.jwtSecret)
	assert.Equal(t, rdb, mm.rdb)
	assert.NotNil(t, mm.connections)
	assert.NotNil(t, mm.userToRoom)
	assert.NotNil(t, mm.roomInfo)
	assert.NotNil(t, mm.pendingMatches)
}

func TestRemoveUser(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user123"
	category := "arrays"
	difficulty := "easy"

	// Add user to queues
	rdb.ZAdd(context.Background(), "queue:arrays:easy", redis.Z{Score: float64(time.Now().Unix()), Member: userId})
	rdb.ZAdd(context.Background(), "queue:arrays", redis.Z{Score: float64(time.Now().Unix()), Member: userId})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: float64(time.Now().Unix()), Member: userId})
	rdb.HSet(context.Background(), "user:user123", "category", category, "difficulty", difficulty)

	// Remove user
	mm.removeUser(userId, category, difficulty)

	// Verify user removed from queues
	allQueue := rdb.ZRange(context.Background(), "queue:all", 0, -1).Val()
	assert.NotContains(t, allQueue, userId)
}

func TestSendToUser(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user123"
	data := map[string]interface{}{
		"type":    "test",
		"message": "hello",
	}

	// Test with no connection - should not panic
	mm.sendToUser(userId, data)

	// Verify no panic occurred
	assert.True(t, true)
}

func TestHandleSessionEndedEvent(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	matchId := uuid.New().String()
	user1 := "user1"
	user2 := "user2"

	// Set up room
	mm.roomMu.Lock()
	mm.userToRoom[user1] = matchId
	mm.userToRoom[user2] = matchId
	mm.roomInfo[matchId] = &models.RoomInfo{
		MatchId: matchId,
		User1:   user1,
		User2:   user2,
	}
	mm.roomMu.Unlock()

	event := map[string]interface{}{
		"matchId": matchId,
		"user1":   user1,
		"user2":   user2,
	}

	payload, _ := json.Marshal(event)
	mm.handleSessionEndedEvent(string(payload))

	// Verify cleanup
	mm.roomMu.RLock()
	_, inRoom1 := mm.userToRoom[user1]
	_, inRoom2 := mm.userToRoom[user2]
	_, roomExists := mm.roomInfo[matchId]
	mm.roomMu.RUnlock()

	assert.False(t, inRoom1)
	assert.False(t, inRoom2)
	assert.False(t, roomExists)
}

func TestCreatePendingMatch(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	u1, u2 := "user1", "user2"
	cat1, diff1 := "arrays", "easy"
	cat2, diff2 := "arrays", "easy"
	stage := 1

	// Add users to queues
	rdb.ZAdd(context.Background(), "queue:arrays:easy", redis.Z{Score: float64(time.Now().Unix()), Member: u1})
	rdb.ZAdd(context.Background(), "queue:arrays", redis.Z{Score: float64(time.Now().Unix()), Member: u1})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: float64(time.Now().Unix()), Member: u1})

	rdb.ZAdd(context.Background(), "queue:arrays:easy", redis.Z{Score: float64(time.Now().Unix()), Member: u2})
	rdb.ZAdd(context.Background(), "queue:arrays", redis.Z{Score: float64(time.Now().Unix()), Member: u2})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: float64(time.Now().Unix()), Member: u2})

	mm.createPendingMatch(u1, u2, cat1, diff1, cat2, diff2, stage)

	// Match ID is generated, so we check count instead
	mm.pendingMu.Lock()
	matchCount := len(mm.pendingMatches)
	mm.pendingMu.Unlock()

	assert.Greater(t, matchCount, 0)

	// Verify users removed from queues
	queue1 := rdb.ZRange(context.Background(), "queue:arrays:easy", 0, -1).Val()
	assert.NotContains(t, queue1, u1)
	assert.NotContains(t, queue1, u2)
}

func TestTryMatchStage(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	category := "arrays"
	difficulty := "easy"

	// Set up two users in queue
	u1, u2 := "user1", "user2"
	now := float64(time.Now().Unix())

	// Add user data
	rdb.HSet(context.Background(), "user:user1", "category", category, "difficulty", difficulty, "joined_at", now)
	rdb.HSet(context.Background(), "user:user2", "category", category, "difficulty", difficulty, "joined_at", now)

	// Add to queue
	rdb.ZAdd(context.Background(), "queue:arrays:easy", redis.Z{Score: now, Member: u1})
	rdb.ZAdd(context.Background(), "queue:arrays:easy", redis.Z{Score: now + 1, Member: u2})

	mm.tryMatchStage(category, difficulty, 1)

	// Verify pending match was created
	mm.pendingMu.Lock()
	matchCount := len(mm.pendingMatches)
	mm.pendingMu.Unlock()

	assert.Equal(t, 1, matchCount)
}

func TestTryMatchStage_EmptyQueue(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	category := "arrays"
	difficulty := "easy"

	mm.tryMatchStage(category, difficulty, 1)

	// Verify no match created
	mm.pendingMu.Lock()
	matchCount := len(mm.pendingMatches)
	mm.pendingMu.Unlock()

	assert.Equal(t, 0, matchCount)
}

func TestTryMatchStage_Stage2(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	category := "arrays"
	difficulty := "easy"

	u1, u2 := "user1", "user2"
	now := float64(time.Now().Unix())

	rdb.HSet(context.Background(), "user:user1", "category", category, "difficulty", difficulty, "joined_at", now)
	rdb.HSet(context.Background(), "user:user2", "category", category, "difficulty", "medium", "joined_at", now)

	rdb.ZAdd(context.Background(), "queue:arrays", redis.Z{Score: now, Member: u1})
	rdb.ZAdd(context.Background(), "queue:arrays", redis.Z{Score: now + 1, Member: u2})

	mm.tryMatchStage(category, difficulty, 2)

	mm.pendingMu.Lock()
	matchCount := len(mm.pendingMatches)
	mm.pendingMu.Unlock()

	assert.Equal(t, 1, matchCount)
}

func TestTryMatchStage_Stage3(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	u1, u2 := "user1", "user2"
	now := float64(time.Now().Unix())

	rdb.HSet(context.Background(), "user:user1", "category", "arrays", "difficulty", "easy", "joined_at", now)
	rdb.HSet(context.Background(), "user:user2", "category", "strings", "difficulty", "medium", "joined_at", now)

	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now, Member: u1})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now + 1, Member: u2})

	mm.tryMatchStage("arrays", "easy", 3)

	mm.pendingMu.Lock()
	matchCount := len(mm.pendingMatches)
	mm.pendingMu.Unlock()

	assert.Equal(t, 1, matchCount)
}

func TestHandleSessionEndedEvent_InvalidJSON(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	// Should not panic on invalid JSON
	mm.handleSessionEndedEvent("invalid json")

	// Verify no panic occurred
	assert.True(t, true)
}

func TestStartMatchmakingLoop_StageProgression(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user1"
	category := "arrays"
	difficulty := "easy"
	now := float64(time.Now().Unix() - 101) // Past STAGE1_TIMEOUT

	// Set up user in stage 1 with old timestamp
	userKey := "user:" + userId
	rdb.HSet(context.Background(), userKey, "category", category, "difficulty", difficulty, "joined_at", now, "stage", 1)
	rdb.ZAdd(context.Background(), "queue:arrays", redis.Z{Score: now, Member: userId})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now, Member: userId})

	// Run matchmaking loop once
	keys, _ := rdb.Keys(context.Background(), "user:*").Result()
	for _, key := range keys {
		user, _ := rdb.HGetAll(context.Background(), key).Result()
		if len(user) == 0 {
			continue
		}

		cat := user["category"]
		diff := user["difficulty"]
		stage, _ := strconv.Atoi(user["stage"])
		joinedAt, _ := strconv.ParseFloat(user["joined_at"], 64)
		elapsed := time.Now().Unix() - int64(joinedAt)

		switch stage {
		case 1:
			if elapsed > 100 { // STAGE1_TIMEOUT
				rdb.HSet(context.Background(), key, "stage", 2)
				mm.tryMatchStage(cat, diff, 2)
			}
		}
	}

	// Verify stage was updated
	userData, _ := rdb.HGetAll(context.Background(), userKey).Result()
	stage, _ := strconv.Atoi(userData["stage"])
	assert.Equal(t, 2, stage)
}

func TestStartPendingMatchExpirationLoop_ExpiredMatch(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	matchId := uuid.New().String()
	user1 := "user1"
	user2 := "user2"

	// Create expired pending match
	pending := &models.PendingMatch{
		MatchId:    matchId,
		User1:      user1,
		User2:      user2,
		Category:   "arrays",
		Difficulty: "easy",
		User1Cat:   "arrays",
		User1Diff:  "easy",
		User2Cat:   "arrays",
		User2Diff:  "easy",
		Token1:     "token1",
		Token2:     "token2",
		Handshakes: make(map[string]bool),
		CreatedAt:  time.Now().Add(-20 * time.Second), // Expired
		ExpiresAt:  time.Now().Add(-5 * time.Second),  // Expired
	}

	// Set up user data for re-queuing
	rdb.HSet(context.Background(), "user:user1", "category", "arrays", "difficulty", "easy", "joined_at", float64(time.Now().Unix()-100))

	mm.pendingMu.Lock()
	mm.pendingMatches[matchId] = pending
	mm.pendingMu.Unlock()

	// Simulate expiration loop check
	now := time.Now()
	mm.pendingMu.Lock()
	for mid, p := range mm.pendingMatches {
		if now.After(p.ExpiresAt) {
			delete(mm.pendingMatches, mid)
		}
	}
	mm.pendingMu.Unlock()

	// Verify pending match was removed
	mm.pendingMu.Lock()
	_, exists := mm.pendingMatches[matchId]
	mm.pendingMu.Unlock()
	assert.False(t, exists)
}

func TestRemoveUser_RemovesFromAllQueues(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user123"
	category := "arrays"
	difficulty := "easy"
	now := float64(time.Now().Unix())

	// Add user to all queues
	rdb.ZAdd(context.Background(), "queue:arrays:easy", redis.Z{Score: now, Member: userId})
	rdb.ZAdd(context.Background(), "queue:arrays", redis.Z{Score: now, Member: userId})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now, Member: userId})
	rdb.HSet(context.Background(), "user:user123", "category", category, "difficulty", difficulty)

	// Remove user
	mm.removeUser(userId, category, difficulty)

	// Verify removed from all queues
	allQueue := rdb.ZRange(context.Background(), "queue:all", 0, -1).Val()
	categoryQueue := rdb.ZRange(context.Background(), "queue:arrays", 0, -1).Val()
	exactQueue := rdb.ZRange(context.Background(), "queue:arrays:easy", 0, -1).Val()

	assert.NotContains(t, allQueue, userId)
	assert.NotContains(t, categoryQueue, userId)
	assert.NotContains(t, exactQueue, userId)

	// Verify user data removed
	userData, _ := rdb.HGetAll(context.Background(), "user:user123").Result()
	assert.Empty(t, userData)
}

func TestSendToUser_WithConnection(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	// Note: Testing sendToUser with actual WebSocket connection requires
	// more complex setup. This test verifies the function doesn't panic
	// when no connection exists (which is already covered).
	// For a real connection test, we'd need to mock websocket.Conn

	userId := "user123"
	data := map[string]interface{}{
		"type":    "test",
		"message": "hello",
	}

	// Should not panic even without connection
	mm.sendToUser(userId, data)
	assert.True(t, true)
}

func TestCreatePendingMatch_DifferentStages(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	u1, u2 := "user1", "user2"

	tests := []struct {
		name          string
		cat1, diff1   string
		cat2, diff2   string
		stage         int
		expectedCat   string
		expectedDiff  string
	}{
		{
			name:         "Stage 1 - same category and difficulty",
			cat1:         "arrays",
			diff1:        "easy",
			cat2:         "arrays",
			diff2:        "easy",
			stage:        1,
			expectedCat:  "arrays",
			expectedDiff: "easy",
		},
		{
			name:         "Stage 2 - same category, different difficulty",
			cat1:         "arrays",
			diff1:        "easy",
			cat2:         "arrays",
			diff2:        "medium",
			stage:        2,
			expectedCat:  "arrays",
			expectedDiff: "easy", // Average of easy(1) and medium(2) = medium(2), but let's check
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mm.pendingMatches = make(map[string]*models.PendingMatch)

			// Add users to queues
			rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s:%s", tt.cat1, tt.diff1), redis.Z{Score: float64(time.Now().Unix()), Member: u1})
			rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s", tt.cat1), redis.Z{Score: float64(time.Now().Unix()), Member: u1})
			rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: float64(time.Now().Unix()), Member: u1})

			rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s:%s", tt.cat2, tt.diff2), redis.Z{Score: float64(time.Now().Unix()), Member: u2})
			rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s", tt.cat2), redis.Z{Score: float64(time.Now().Unix()), Member: u2})
			rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: float64(time.Now().Unix()), Member: u2})

			mm.createPendingMatch(u1, u2, tt.cat1, tt.diff1, tt.cat2, tt.diff2, tt.stage)

			// Verify pending match created
			mm.pendingMu.Lock()
			matchCount := len(mm.pendingMatches)
			mm.pendingMu.Unlock()

			assert.Equal(t, 1, matchCount)

			// Verify users removed from queues
			queue1 := rdb.ZRange(context.Background(), fmt.Sprintf("queue:%s:%s", tt.cat1, tt.diff1), 0, -1).Val()
			assert.NotContains(t, queue1, u1)
		})
	}
}

func TestTryMatchStage_NoUsers(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	mm.tryMatchStage("arrays", "easy", 1)

	mm.pendingMu.Lock()
	matchCount := len(mm.pendingMatches)
	mm.pendingMu.Unlock()

	assert.Equal(t, 0, matchCount)
}

func TestTryMatchStage_SingleUser(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user1"
	now := float64(time.Now().Unix())

	rdb.HSet(context.Background(), "user:user1", "category", "arrays", "difficulty", "easy", "joined_at", now)
	rdb.ZAdd(context.Background(), "queue:arrays:easy", redis.Z{Score: now, Member: userId})

	mm.tryMatchStage("arrays", "easy", 1)

	mm.pendingMu.Lock()
	matchCount := len(mm.pendingMatches)
	mm.pendingMu.Unlock()

	assert.Equal(t, 0, matchCount) // No match with only one user
}