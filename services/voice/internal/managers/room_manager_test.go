package managers

import (
	"context"
	"testing"
	"time"
	"voice/internal/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return mr, client
}

func TestNewRoomManager(t *testing.T) {
	mr, _ := setupTestRedis(t)
	defer mr.Close()

	rm := NewRoomManager(mr.Addr())

	if rm.rdb == nil {
		t.Error("Redis client should be initialized")
	}

	if rm.roomStatusMap == nil {
		t.Error("Room status map should be initialized")
	}

	if rm.rooms == nil {
		t.Error("Rooms map should be initialized")
	}

	if rm.instanceID == "" {
		t.Error("Instance ID should be set")
	}
}

func TestRoomManager_GetOrCreateRoom(t *testing.T) {
	mr, _ := setupTestRedis(t)
	defer mr.Close()

	rm := NewRoomManager(mr.Addr())

	roomID := "test-room-123"

	// First call should create room
	room1 := rm.GetOrCreateRoom(roomID)
	if room1 == nil {
		t.Fatal("Room should be created")
	}

	if room1.ID != roomID {
		t.Errorf("Expected room ID %s, got %s", roomID, room1.ID)
	}

	// Second call should return same room
	room2 := rm.GetOrCreateRoom(roomID)
	if room2 != room1 {
		t.Error("Should return the same room instance")
	}

	// Check that room is in the map
	rm.mu.RLock()
	_, exists := rm.rooms[roomID]
	rm.mu.RUnlock()

	if !exists {
		t.Error("Room should exist in rooms map")
	}
}

func TestRoomManager_DeleteRoom(t *testing.T) {
	mr, _ := setupTestRedis(t)
	defer mr.Close()

	rm := NewRoomManager(mr.Addr())

	roomID := "test-room-123"

	// Create room
	rm.GetOrCreateRoom(roomID)

	// Verify it exists
	rm.mu.RLock()
	_, exists := rm.rooms[roomID]
	rm.mu.RUnlock()
	if !exists {
		t.Fatal("Room should exist before deletion")
	}

	// Delete room
	rm.DeleteRoom(roomID)

	// Verify it's deleted
	rm.mu.RLock()
	_, exists = rm.rooms[roomID]
	rm.mu.RUnlock()
	if exists {
		t.Error("Room should not exist after deletion")
	}
}

func TestRoomManager_PublishPresenceEvent(t *testing.T) {
	mr, _ := setupTestRedis(t)
	defer mr.Close()

	rm := NewRoomManager(mr.Addr())

	event := &models.PresenceEvent{
		Type:     "user-joined",
		RoomID:   "room123",
		UserID:   "user1",
		Username: "Alice",
		IsMuted:  false,
	}

	err := rm.PublishPresenceEvent(event)
	if err != nil {
		t.Errorf("Failed to publish presence event: %v", err)
	}

	if event.InstanceID != rm.instanceID {
		t.Error("InstanceID should be set by PublishPresenceEvent")
	}

	if event.Timestamp.IsZero() {
		t.Error("Timestamp should be set by PublishPresenceEvent")
	}
}

func TestRoomManager_GetRoomStatus_FromCache(t *testing.T) {
	mr, _ := setupTestRedis(t)
	defer mr.Close()

	rm := NewRoomManager(mr.Addr())

	matchID := "match123"
	roomInfo := &models.RoomInfo{
		MatchId:    matchID,
		User1:      "user1",
		User2:      "user2",
		Category:   "algorithms",
		Difficulty: "easy",
		Status:     "ready",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	// Add to cache
	rm.mu.Lock()
	rm.roomStatusMap[matchID] = roomInfo
	rm.mu.Unlock()

	// Retrieve from cache
	retrieved, err := rm.GetRoomStatus(matchID)
	if err != nil {
		t.Errorf("Failed to get room status: %v", err)
	}

	if retrieved.MatchId != matchID {
		t.Errorf("Expected MatchId %s, got %s", matchID, retrieved.MatchId)
	}

	if retrieved.User1 != "user1" {
		t.Errorf("Expected User1 'user1', got %s", retrieved.User1)
	}
}

func TestRoomManager_GetRoomStatus_FromRedis(t *testing.T) {
	mr, client := setupTestRedis(t)
	defer mr.Close()

	rm := NewRoomManager(mr.Addr())

	matchID := "match456"
	ctx := context.Background()

	// Store in Redis
	roomKey := "room:" + matchID
	client.HSet(ctx, roomKey, map[string]interface{}{
		"matchId":    matchID,
		"user1":      "alice",
		"user2":      "bob",
		"category":   "algorithms",
		"difficulty": "medium",
		"status":     "ready",
		"createdAt":  time.Now().Format(time.RFC3339),
		"token1":     "token1",
		"token2":     "token2",
	})

	// Retrieve from Redis
	retrieved, err := rm.GetRoomStatus(matchID)
	if err != nil {
		t.Errorf("Failed to get room status from Redis: %v", err)
	}

	if retrieved.MatchId != matchID {
		t.Errorf("Expected MatchId %s, got %s", matchID, retrieved.MatchId)
	}

	if retrieved.User1 != "alice" {
		t.Errorf("Expected User1 'alice', got %s", retrieved.User1)
	}

	if retrieved.User2 != "bob" {
		t.Errorf("Expected User2 'bob', got %s", retrieved.User2)
	}

	// Should now be in cache
	rm.mu.RLock()
	_, exists := rm.roomStatusMap[matchID]
	rm.mu.RUnlock()

	if !exists {
		t.Error("Room status should be cached after fetching from Redis")
	}
}

func TestRoomManager_GetRoomStatus_NotFound(t *testing.T) {
	mr, _ := setupTestRedis(t)
	defer mr.Close()

	rm := NewRoomManager(mr.Addr())

	_, err := rm.GetRoomStatus("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent room")
	}
}

func TestRoomManager_ConcurrentRoomOperations(t *testing.T) {
	mr, _ := setupTestRedis(t)
	defer mr.Close()

	rm := NewRoomManager(mr.Addr())
	done := make(chan bool)

	// Concurrent creates
	go func() {
		for i := 0; i < 50; i++ {
			rm.GetOrCreateRoom("room-a")
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			rm.GetOrCreateRoom("room-b")
		}
		done <- true
	}()

	// Concurrent deletes
	go func() {
		time.Sleep(10 * time.Millisecond)
		for i := 0; i < 25; i++ {
			rm.DeleteRoom("room-a")
			rm.GetOrCreateRoom("room-a")
		}
		done <- true
	}()

	<-done
	<-done
	<-done

	// Should have 2 rooms at the end
	rm.mu.RLock()
	roomCount := len(rm.rooms)
	rm.mu.RUnlock()

	if roomCount != 2 {
		t.Errorf("Expected 2 rooms, got %d", roomCount)
	}
}

func TestCloneRoomInfo(t *testing.T) {
	original := &models.RoomInfo{
		MatchId:    "match123",
		User1:      "user1",
		User2:      "user2",
		Category:   "algorithms",
		Difficulty: "easy",
		Status:     "ready",
		Token1:     "token1",
		Token2:     "token2",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	cloned := cloneRoomInfo(original)

	if cloned == original {
		t.Error("Clone should be a different instance")
	}

	if cloned.MatchId != original.MatchId {
		t.Error("Cloned MatchId should match original")
	}

	if cloned.User1 != original.User1 {
		t.Error("Cloned User1 should match original")
	}

	nilClone := cloneRoomInfo(nil)
	if nilClone != nil {
		t.Error("Cloning nil should return nil")
	}
}

func TestRoomManager_MultipleInstances(t *testing.T) {
	mr, _ := setupTestRedis(t)
	defer mr.Close()

	rm1 := NewRoomManager(mr.Addr())
	rm2 := NewRoomManager(mr.Addr())

	if rm1.instanceID == rm2.instanceID {
		t.Error("Different instances should have different IDs")
	}

	// Each instance should have its own room map
	room1 := rm1.GetOrCreateRoom("room-test")
	room2 := rm2.GetOrCreateRoom("room-test")

	// Same room ID but different instances
	if room1 == room2 {
		t.Error("Different RoomManager instances should have separate room instances")
	}

	// But both should have the same room ID
	if room1.ID != room2.ID {
		t.Error("Rooms should have the same ID")
	}
}
