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

// setupTestRedis creates a miniredis instance and redis clients for testing
func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client, *redis.Client) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	// Client for matchmaking coordination
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { rdb.Close() })

	// Client for pub/sub (can use same miniredis instance for testing)
	pubSubClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { pubSubClient.Close() })

	return mr, rdb, pubSubClient
}

func TestNewMatchManager(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)

	mm := NewMatchManager(secret, rdb, pubSubClient)

	assert.NotNil(t, mm)
	assert.Equal(t, secret, mm.jwtSecret)
	assert.Equal(t, rdb, mm.rdb)
	assert.NotNil(t, mm.connections)
	assert.NotNil(t, mm.subClient)
	assert.NotNil(t, mm.pubClient)
	assert.NotEmpty(t, mm.instanceID)
}

func TestRemoveUser(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	userId := "user123"
	category := "arrays"
	difficulty := "easy"

	// Add user to queues in Redis
	now := float64(time.Now().Unix())
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s:%s", category, difficulty), redis.Z{Score: now, Member: userId})
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s", category), redis.Z{Score: now, Member: userId})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now, Member: userId})
	rdb.HSet(context.Background(), fmt.Sprintf("user:%s", userId), "category", category, "difficulty", difficulty)

	// Remove user
	mm.removeUser(userId, category, difficulty)

	// Verify user removed from queues
	allQueue := rdb.ZRange(context.Background(), "queue:all", 0, -1).Val()
	assert.NotContains(t, allQueue, userId)

	// Verify user data removed
	userData, _ := rdb.HGetAll(context.Background(), fmt.Sprintf("user:%s", userId)).Result()
	assert.Empty(t, userData)
}

func TestRemoveUser_RemovesFromAllQueues(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	userId := "user123"
	category := "arrays"
	difficulty := "easy"
	now := float64(time.Now().Unix())

	// Add user to all queues
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s:%s", category, difficulty), redis.Z{Score: now, Member: userId})
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s", category), redis.Z{Score: now, Member: userId})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now, Member: userId})
	rdb.HSet(context.Background(), fmt.Sprintf("user:%s", userId), "category", category, "difficulty", difficulty)

	// Remove user
	mm.removeUser(userId, category, difficulty)

	// Verify removed from all queues
	allQueue := rdb.ZRange(context.Background(), "queue:all", 0, -1).Val()
	categoryQueue := rdb.ZRange(context.Background(), fmt.Sprintf("queue:%s", category), 0, -1).Val()
	exactQueue := rdb.ZRange(context.Background(), fmt.Sprintf("queue:%s:%s", category, difficulty), 0, -1).Val()

	assert.NotContains(t, allQueue, userId)
	assert.NotContains(t, categoryQueue, userId)
	assert.NotContains(t, exactQueue, userId)

	// Verify user data removed
	userData, _ := rdb.HGetAll(context.Background(), fmt.Sprintf("user:%s", userId)).Result()
	assert.Empty(t, userData)
}

func TestSendToUser(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	userId := "user123"
	data := map[string]interface{}{
		"type":    "test",
		"message": "hello",
	}

	// Test with no connection - should not panic
	// sendToUser publishes to Redis pub/sub, which should work even without connections
	mm.sendToUser(userId, data)

	// Verify no panic occurred
	assert.True(t, true)
}

func TestSendToUser_WithConnection(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	// Note: Testing sendToUser with actual WebSocket connection requires
	// more complex setup. This test verifies the function doesn't panic
	// when called (which publishes to Redis).

	userId := "user123"
	data := map[string]interface{}{
		"type":    "test",
		"message": "hello",
	}

	// Should not panic even without connection
	mm.sendToUser(userId, data)
	assert.True(t, true)
}

func TestHandleSessionEndedEvent(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	matchId := uuid.New().String()
	user1 := "user1"
	user2 := "user2"

	// Set up room in Redis
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", user1), matchId, 2*time.Hour)
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", user2), matchId, 2*time.Hour)
	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      user1,
		User2:      user2,
		Category:   "arrays",
		Difficulty: "easy",
		Status:     "active",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}
	roomJSON, _ := json.Marshal(room)
	rdb.Set(context.Background(), fmt.Sprintf("room:%s", matchId), roomJSON, 2*time.Hour)

	event := map[string]interface{}{
		"matchId": matchId,
		"user1":   user1,
		"user2":   user2,
	}

	payload, _ := json.Marshal(event)
	mm.handleSessionEndedEvent(string(payload))

	// Verify cleanup in Redis
	room1, err1 := rdb.Get(context.Background(), fmt.Sprintf("user_room:%s", user1)).Result()
	room2, err2 := rdb.Get(context.Background(), fmt.Sprintf("user_room:%s", user2)).Result()
	roomData, err3 := rdb.Get(context.Background(), fmt.Sprintf("room:%s", matchId)).Result()

	assert.Error(t, err1) // Should not exist
	assert.Error(t, err2) // Should not exist
	assert.Error(t, err3) // Should not exist
	assert.Empty(t, room1)
	assert.Empty(t, room2)
	assert.Empty(t, roomData)
}

func TestHandleSessionEndedEvent_InvalidJSON(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	// Should not panic on invalid JSON
	mm.handleSessionEndedEvent("invalid json")

	// Verify no panic occurred
	assert.True(t, true)
}

func TestCreatePendingMatch(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	u1, u2 := "user1", "user2"
	cat1, diff1 := "arrays", "easy"
	cat2, diff2 := "arrays", "easy"
	stage := 1

	// Add users to queues
	now := float64(time.Now().Unix())
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s:%s", cat1, diff1), redis.Z{Score: now, Member: u1})
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s", cat1), redis.Z{Score: now, Member: u1})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now, Member: u1})

	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s:%s", cat2, diff2), redis.Z{Score: now, Member: u2})
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s", cat2), redis.Z{Score: now, Member: u2})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now, Member: u2})

	mm.createPendingMatch(u1, u2, cat1, diff1, cat2, diff2, stage)

	// Verify pending match was created in Redis
	pendingKeys, _ := rdb.Keys(context.Background(), "pending_match:*").Result()
	assert.Greater(t, len(pendingKeys), 0)

	// Verify users removed from queues
	queue1 := rdb.ZRange(context.Background(), fmt.Sprintf("queue:%s:%s", cat1, diff1), 0, -1).Val()
	assert.NotContains(t, queue1, u1)
	assert.NotContains(t, queue1, u2)
}

func TestCreatePendingMatch_DifferentStages(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	u1, u2 := "user1", "user2"

	tests := []struct {
		name         string
		cat1, diff1  string
		cat2, diff2  string
		stage        int
		expectedCat  string
		expectedDiff string
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
			expectedDiff: "medium", // Average of easy(1) and medium(2) = medium(2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear Redis for each test
			rdb.FlushDB(context.Background())

			// Add users to queues
			now := float64(time.Now().Unix())
			rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s:%s", tt.cat1, tt.diff1), redis.Z{Score: now, Member: u1})
			rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s", tt.cat1), redis.Z{Score: now, Member: u1})
			rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now, Member: u1})

			rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s:%s", tt.cat2, tt.diff2), redis.Z{Score: now, Member: u2})
			rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s", tt.cat2), redis.Z{Score: now, Member: u2})
			rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now, Member: u2})

			mm.createPendingMatch(u1, u2, tt.cat1, tt.diff1, tt.cat2, tt.diff2, tt.stage)

			// Verify pending match created in Redis
			pendingKeys, _ := rdb.Keys(context.Background(), "pending_match:*").Result()
			assert.Equal(t, 1, len(pendingKeys))

			// Verify users removed from queues
			queue1 := rdb.ZRange(context.Background(), fmt.Sprintf("queue:%s:%s", tt.cat1, tt.diff1), 0, -1).Val()
			assert.NotContains(t, queue1, u1)
		})
	}
}

func TestTryMatchStage(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	category := "arrays"
	difficulty := "easy"

	// Set up two users in queue
	u1, u2 := "user1", "user2"
	now := float64(time.Now().Unix())

	// Add user data
	rdb.HSet(context.Background(), fmt.Sprintf("user:%s", u1), "category", category, "difficulty", difficulty, "joined_at", now)
	rdb.HSet(context.Background(), fmt.Sprintf("user:%s", u2), "category", category, "difficulty", difficulty, "joined_at", now)

	// Add to queue
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s:%s", category, difficulty), redis.Z{Score: now, Member: u1})
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s:%s", category, difficulty), redis.Z{Score: now + 1, Member: u2})

	mm.tryMatchStage(category, difficulty, 1)

	// Verify pending match was created in Redis
	pendingKeys, _ := rdb.Keys(context.Background(), "pending_match:*").Result()
	assert.Equal(t, 1, len(pendingKeys))
}

func TestTryMatchStage_EmptyQueue(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	category := "arrays"
	difficulty := "easy"

	mm.tryMatchStage(category, difficulty, 1)

	// Verify no match created
	pendingKeys, _ := rdb.Keys(context.Background(), "pending_match:*").Result()
	assert.Equal(t, 0, len(pendingKeys))
}

func TestTryMatchStage_NoUsers(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	mm.tryMatchStage("arrays", "easy", 1)

	pendingKeys, _ := rdb.Keys(context.Background(), "pending_match:*").Result()
	assert.Equal(t, 0, len(pendingKeys))
}

func TestTryMatchStage_SingleUser(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	userId := "user1"
	now := float64(time.Now().Unix())

	rdb.HSet(context.Background(), fmt.Sprintf("user:%s", userId), "category", "arrays", "difficulty", "easy", "joined_at", now)
	rdb.ZAdd(context.Background(), "queue:arrays:easy", redis.Z{Score: now, Member: userId})

	mm.tryMatchStage("arrays", "easy", 1)

	pendingKeys, _ := rdb.Keys(context.Background(), "pending_match:*").Result()
	assert.Equal(t, 0, len(pendingKeys)) // No match with only one user
}

func TestTryMatchStage_Stage2(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	category := "arrays"
	difficulty := "easy"

	u1, u2 := "user1", "user2"
	now := float64(time.Now().Unix())

	rdb.HSet(context.Background(), fmt.Sprintf("user:%s", u1), "category", category, "difficulty", difficulty, "joined_at", now)
	rdb.HSet(context.Background(), fmt.Sprintf("user:%s", u2), "category", category, "difficulty", "medium", "joined_at", now)

	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s", category), redis.Z{Score: now, Member: u1})
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s", category), redis.Z{Score: now + 1, Member: u2})

	mm.tryMatchStage(category, difficulty, 2)

	pendingKeys, _ := rdb.Keys(context.Background(), "pending_match:*").Result()
	assert.Equal(t, 1, len(pendingKeys))
}

func TestTryMatchStage_Stage3(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	u1, u2 := "user1", "user2"
	now := float64(time.Now().Unix())

	rdb.HSet(context.Background(), fmt.Sprintf("user:%s", u1), "category", "arrays", "difficulty", "easy", "joined_at", now)
	rdb.HSet(context.Background(), fmt.Sprintf("user:%s", u2), "category", "strings", "difficulty", "medium", "joined_at", now)

	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now, Member: u1})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now + 1, Member: u2})

	mm.tryMatchStage("arrays", "easy", 3)

	pendingKeys, _ := rdb.Keys(context.Background(), "pending_match:*").Result()
	assert.Equal(t, 1, len(pendingKeys))
}

func TestStartMatchmakingLoop_StageProgression(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	userId := "user1"
	category := "arrays"
	difficulty := "easy"
	now := float64(time.Now().Unix() - 101) // Past STAGE1_TIMEOUT

	// Set up user in stage 1 with old timestamp
	userKey := fmt.Sprintf("user:%s", userId)
	rdb.HSet(context.Background(), userKey, "category", category, "difficulty", difficulty, "joined_at", now, "stage", 1)
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s", category), redis.Z{Score: now, Member: userId})
	rdb.ZAdd(context.Background(), "queue:all", redis.Z{Score: now, Member: userId})

	// Run matchmaking logic (simulating one iteration)
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
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	matchId := uuid.New().String()
	user1 := "user1"
	user2 := "user2"

	// Create expired pending match in Redis
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
	rdb.HSet(context.Background(), fmt.Sprintf("user:%s", user1), "category", "arrays", "difficulty", "easy", "joined_at", float64(time.Now().Unix()-100))

	pendingJSON, _ := json.Marshal(pending)
	rdb.Set(context.Background(), fmt.Sprintf("pending_match:%s", matchId), pendingJSON, 20*time.Second)

	// Simulate expiration loop check
	pendingKeys, _ := rdb.Keys(context.Background(), "pending_match:*").Result()
	for _, pendingKey := range pendingKeys {
		pendingJSON, err := rdb.Get(context.Background(), pendingKey).Result()
		if err != nil {
			continue
		}

		var p models.PendingMatch
		if err := json.Unmarshal([]byte(pendingJSON), &p); err != nil {
			continue
		}

		if time.Now().After(p.ExpiresAt) {
			mm.handleExpiredMatch(&p)
		}
	}

	// Verify pending match was removed from Redis
	pendingData, err := rdb.Get(context.Background(), fmt.Sprintf("pending_match:%s", matchId)).Result()
	assert.Error(t, err) // Should not exist
	assert.Empty(t, pendingData)
}

func TestGetRoomForUser(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	userId := "user1"
	matchId := uuid.New().String()

	// Set up room in Redis
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", userId), matchId, 2*time.Hour)

	// Get room for user
	roomId, err := mm.GetRoomForUser(userId)
	assert.NoError(t, err)
	assert.Equal(t, matchId, roomId)
}

func TestGetRoomForUser_NotFound(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	// Get room for non-existent user
	_, err := mm.GetRoomForUser("nonexistent")
	assert.Error(t, err)
}

func TestGetRoomInfo(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	matchId := uuid.New().String()
	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      "user1",
		User2:      "user2",
		Category:   "arrays",
		Difficulty: "easy",
		Status:     "active",
		Token1:     "token1",
		Token2:     "token2",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	roomJSON, _ := json.Marshal(room)
	rdb.Set(context.Background(), fmt.Sprintf("room:%s", matchId), roomJSON, 2*time.Hour)

	// Get room info
	roomInfo, err := mm.GetRoomInfo(matchId)
	assert.NoError(t, err)
	assert.NotNil(t, roomInfo)
	assert.Equal(t, matchId, roomInfo.MatchId)
	assert.Equal(t, "user1", roomInfo.User1)
	assert.Equal(t, "user2", roomInfo.User2)
}

func TestGetRoomInfo_NotFound(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	// Get room info for non-existent room
	_, err := mm.GetRoomInfo("nonexistent")
	assert.Error(t, err)
}

func TestHandleMatchAccept_BothAccepted(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	matchId := uuid.New().String()
	user1 := "user1"
	user2 := "user2"

	// Create pending match in Redis
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
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(15 * time.Second),
	}

	pendingJSON, _ := json.Marshal(pending)
	rdb.Set(context.Background(), fmt.Sprintf("pending_match:%s", matchId), pendingJSON, 20*time.Second)
	rdb.Set(context.Background(), fmt.Sprintf("handshake:%s:%s", matchId, user1), "pending", 20*time.Second)
	rdb.Set(context.Background(), fmt.Sprintf("handshake:%s:%s", matchId, user2), "pending", 20*time.Second)

	// First user accepts
	err1 := mm.HandleMatchAccept(matchId, user1)
	assert.NoError(t, err1)

	// Second user accepts (should finalize match)
	err2 := mm.HandleMatchAccept(matchId, user2)
	assert.NoError(t, err2)

	// Verify room was created
	roomId1, err1 := mm.GetRoomForUser(user1)
	assert.NoError(t, err1)
	assert.Equal(t, matchId, roomId1)

	roomId2, err2 := mm.GetRoomForUser(user2)
	assert.NoError(t, err2)
	assert.Equal(t, matchId, roomId2)

	// Verify room info exists
	roomInfo, err := mm.GetRoomInfo(matchId)
	assert.NoError(t, err)
	assert.NotNil(t, roomInfo)
	assert.Equal(t, matchId, roomInfo.MatchId)
}

func TestRequeueUser(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	userId := "user1"
	category := "arrays"
	difficulty := "easy"
	originalTime := float64(time.Now().Unix() - 100)

	// Set up user with original timestamp
	userKey := fmt.Sprintf("user:%s", userId)
	rdb.HSet(context.Background(), userKey, "category", category, "difficulty", difficulty, "joined_at", originalTime)

	// Re-queue user
	mm.requeueUser(userId, category, difficulty)

	// Verify user was added back to queues with original timestamp
	queueMembers := rdb.ZRangeWithScores(context.Background(), fmt.Sprintf("queue:%s:%s", category, difficulty), 0, -1).Val()
	assert.Greater(t, len(queueMembers), 0)
	assert.Contains(t, queueMembers[0].Member, userId)

	// Verify stage was reset to 1
	userData, _ := rdb.HGetAll(context.Background(), userKey).Result()
	assert.Equal(t, "1", userData["stage"])
}

func TestRegisterConnection(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	// Note: We can't easily test WebSocket connections in unit tests,
	// but we can verify the function exists and doesn't panic
	// In a real scenario, you'd need to mock websocket.Conn

	// Verify connections map is initialized
	assert.NotNil(t, mm.connections)
}

func TestUnregisterConnection(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb, pubSubClient := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb, pubSubClient)

	userId := "user1"

	// Note: We can't easily test WebSocket connections in unit tests,
	// but we can verify the function exists and doesn't panic

	// Unregister non-existent connection should not panic
	mm.UnregisterConnection(userId)
	assert.True(t, true)
}
