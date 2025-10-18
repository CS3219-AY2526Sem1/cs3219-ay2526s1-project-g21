package models

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNewRoom(t *testing.T) {
	roomID := "test-room-123"
	room := NewRoom(roomID)

	if room.ID != roomID {
		t.Errorf("Expected room ID %s, got %s", roomID, room.ID)
	}

	if room.Users == nil {
		t.Error("Expected Users map to be initialized")
	}

	if room.Connections == nil {
		t.Error("Expected Connections map to be initialized")
	}

	if room.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestRoom_AddUser(t *testing.T) {
	room := NewRoom("test-room")
	user := &User{
		ID:       "user1",
		Username: "Alice",
		JoinedAt: time.Now(),
		IsMuted:  false,
		IsDeaf:   false,
	}

	room.AddUser(user)

	if len(room.Users) != 1 {
		t.Errorf("Expected 1 user, got %d", len(room.Users))
	}

	retrievedUser, exists := room.Users[user.ID]
	if !exists {
		t.Error("User should exist in room")
	}

	if retrievedUser.Username != "Alice" {
		t.Errorf("Expected username Alice, got %s", retrievedUser.Username)
	}
}

func TestRoom_RemoveUser(t *testing.T) {
	room := NewRoom("test-room")
	user := &User{
		ID:       "user1",
		Username: "Alice",
		JoinedAt: time.Now(),
	}

	room.AddUser(user)
	if len(room.Users) != 1 {
		t.Fatal("User should be added")
	}

	room.RemoveUser(user.ID)
	if len(room.Users) != 0 {
		t.Errorf("Expected 0 users after removal, got %d", len(room.Users))
	}

	_, exists := room.Users[user.ID]
	if exists {
		t.Error("User should not exist after removal")
	}
}

func TestRoom_GetUser(t *testing.T) {
	room := NewRoom("test-room")
	user := &User{
		ID:       "user1",
		Username: "Alice",
		JoinedAt: time.Now(),
	}

	room.AddUser(user)

	// Test getting existing user
	retrievedUser, exists := room.GetUser(user.ID)
	if !exists {
		t.Error("User should exist")
	}
	if retrievedUser.ID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, retrievedUser.ID)
	}

	// Test getting non-existent user
	_, exists = room.GetUser("nonexistent")
	if exists {
		t.Error("Non-existent user should not be found")
	}
}

func TestRoom_AddConn(t *testing.T) {
	room := NewRoom("test-room")

	// Create a mock connection (nil is acceptable for testing the map)
	var conn *websocket.Conn = nil
	userID := "user1"

	room.AddConn(userID, conn)

	if len(room.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(room.Connections))
	}

	_, exists := room.Connections[userID]
	if !exists {
		t.Error("Connection should exist in room")
	}
}

func TestRoom_RemoveConn(t *testing.T) {
	room := NewRoom("test-room")
	userID := "user1"

	// For testing purposes, we manually add to the map without calling AddConn
	// to avoid issues with closing nil connections
	room.mu.Lock()
	room.Connections[userID] = nil
	room.mu.Unlock()

	if len(room.Connections) != 1 {
		t.Fatal("Connection should be added")
	}

	// RemoveConn should handle nil connections gracefully
	room.RemoveConn(userID)
	if len(room.Connections) != 0 {
		t.Errorf("Expected 0 connections after removal, got %d", len(room.Connections))
	}
}

func TestRoom_GetUserCount(t *testing.T) {
	room := NewRoom("test-room")

	if room.GetUserCount() != 0 {
		t.Errorf("Expected 0 users initially, got %d", room.GetUserCount())
	}

	user1 := &User{ID: "user1", Username: "Alice", JoinedAt: time.Now()}
	user2 := &User{ID: "user2", Username: "Bob", JoinedAt: time.Now()}

	room.AddUser(user1)
	if room.GetUserCount() != 1 {
		t.Errorf("Expected 1 user, got %d", room.GetUserCount())
	}

	room.AddUser(user2)
	if room.GetUserCount() != 2 {
		t.Errorf("Expected 2 users, got %d", room.GetUserCount())
	}

	room.RemoveUser(user1.ID)
	if room.GetUserCount() != 1 {
		t.Errorf("Expected 1 user after removal, got %d", room.GetUserCount())
	}
}

func TestRoom_GetRoomStatus(t *testing.T) {
	room := NewRoom("test-room-123")

	user1 := &User{
		ID:       "user1",
		Username: "Alice",
		IsMuted:  false,
		IsDeaf:   false,
		JoinedAt: time.Now(),
	}
	user2 := &User{
		ID:       "user2",
		Username: "Bob",
		IsMuted:  true,
		IsDeaf:   false,
		JoinedAt: time.Now(),
	}

	room.AddUser(user1)
	room.AddUser(user2)

	status := room.GetRoomStatus()

	if status.Type != "room-status" {
		t.Errorf("Expected type 'room-status', got %s", status.Type)
	}

	if status.RoomID != "test-room-123" {
		t.Errorf("Expected room ID 'test-room-123', got %s", status.RoomID)
	}

	if status.UserCount != 2 {
		t.Errorf("Expected 2 users, got %d", status.UserCount)
	}

	if len(status.Users) != 2 {
		t.Errorf("Expected 2 users in status, got %d", len(status.Users))
	}

	// Check that user info is correctly populated
	var aliceFound, bobFound bool
	for _, userInfo := range status.Users {
		if userInfo.ID == "user1" {
			aliceFound = true
			if userInfo.Username != "Alice" {
				t.Errorf("Expected username Alice, got %s", userInfo.Username)
			}
			if userInfo.IsMuted {
				t.Error("Alice should not be muted")
			}
		}
		if userInfo.ID == "user2" {
			bobFound = true
			if userInfo.Username != "Bob" {
				t.Errorf("Expected username Bob, got %s", userInfo.Username)
			}
			if !userInfo.IsMuted {
				t.Error("Bob should be muted")
			}
		}
	}

	if !aliceFound || !bobFound {
		t.Error("Both users should be in room status")
	}
}

func TestRoom_ConcurrentAccess(t *testing.T) {
	room := NewRoom("test-room")
	done := make(chan bool)

	// Concurrent adds
	go func() {
		for i := 0; i < 100; i++ {
			user := &User{
				ID:       "user-a",
				Username: "Alice",
				JoinedAt: time.Now(),
			}
			room.AddUser(user)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			user := &User{
				ID:       "user-b",
				Username: "Bob",
				JoinedAt: time.Now(),
			}
			room.AddUser(user)
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			room.GetUserCount()
			room.GetRoomStatus()
		}
		done <- true
	}()

	// Wait for all goroutines
	<-done
	<-done
	<-done

	// Should have 2 users (user-a and user-b)
	if room.GetUserCount() != 2 {
		t.Errorf("Expected 2 users after concurrent access, got %d", room.GetUserCount())
	}
}

func TestUser_Creation(t *testing.T) {
	now := time.Now()
	user := &User{
		ID:       "user123",
		Username: "TestUser",
		IsMuted:  false,
		IsDeaf:   false,
		JoinedAt: now,
	}

	if user.ID != "user123" {
		t.Errorf("Expected ID 'user123', got %s", user.ID)
	}

	if user.Username != "TestUser" {
		t.Errorf("Expected username 'TestUser', got %s", user.Username)
	}

	if user.IsMuted {
		t.Error("User should not be muted by default")
	}

	if user.IsDeaf {
		t.Error("User should not be deaf by default")
	}

	if user.JoinedAt != now {
		t.Error("JoinedAt should match the provided time")
	}
}

func TestRoomInfo_Structure(t *testing.T) {
	roomInfo := RoomInfo{
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

	if roomInfo.MatchId != "match123" {
		t.Errorf("Expected MatchId 'match123', got %s", roomInfo.MatchId)
	}

	if roomInfo.Category != "algorithms" {
		t.Errorf("Expected Category 'algorithms', got %s", roomInfo.Category)
	}

	if roomInfo.Status != "ready" {
		t.Errorf("Expected Status 'ready', got %s", roomInfo.Status)
	}
}
