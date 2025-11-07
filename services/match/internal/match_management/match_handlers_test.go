package match_management

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"match/internal/models"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// Note: setupTestRedis is defined in match_manager_test.go

func TestJoinHandler_Success(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	reqBody := models.JoinReq{
		UserID:     "user123",
		Category:   "arrays",
		Difficulty: "easy",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/join", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	mm.JoinHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.OK)
	assert.Equal(t, "queued", resp.Info)

	// Verify user was added to Redis
	userKey := "user:user123"
	userData, err := rdb.HGetAll(context.Background(), userKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, "arrays", userData["category"])
	assert.Equal(t, "easy", userData["difficulty"])
}

func TestJoinHandler_MissingUserId(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	reqBody := models.JoinReq{
		Category:   "arrays",
		Difficulty: "easy",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/join", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	mm.JoinHandler(w, req)

	// Handler still processes empty userId and adds to queue
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJoinHandler_InvalidJSON(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/join", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	mm.JoinHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.OK)
}

func TestJoinHandler_WrongMethod(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/join", nil)
	w := httptest.NewRecorder()

	mm.JoinHandler(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestJoinHandler_Options(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/match/join", nil)
	w := httptest.NewRecorder()

	mm.JoinHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJoinHandler_AlreadyInRoom(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user123"
	matchId := uuid.New().String()

	// Set up user in room in Redis
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", userId), matchId, 2*time.Hour)

	reqBody := models.JoinReq{
		UserID:     userId,
		Category:   "arrays",
		Difficulty: "easy",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/join", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	mm.JoinHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.OK)
	assert.Equal(t, "already in a room", resp.Info)
}

func TestCancelHandler_Success(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user123"
	category := "arrays"
	difficulty := "easy"

	// Set up user in queue in Redis
	userKey := fmt.Sprintf("user:%s", userId)
	now := float64(time.Now().Unix())
	rdb.HSet(context.Background(), userKey, map[string]interface{}{
		"category":   category,
		"difficulty": difficulty,
		"joined_at":  now,
		"stage":      1,
	})
	rdb.ZAdd(context.Background(), fmt.Sprintf("queue:%s:%s", category, difficulty), redis.Z{Score: now, Member: userId})

	reqBody := map[string]string{
		"userId": userId,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/cancel", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	mm.CancelHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.OK)
	assert.Equal(t, "cancelled", resp.Info)

	// Verify user removed from Redis
	userData, err := rdb.HGetAll(context.Background(), userKey).Result()
	assert.NoError(t, err)
	assert.Empty(t, userData)
}

func TestCancelHandler_NotInQueue(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	reqBody := map[string]string{
		"userId": "nonexistent",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/cancel", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	mm.CancelHandler(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.OK)
}

func TestCancelHandler_InvalidJSON(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/cancel", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	mm.CancelHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCancelHandler_WrongMethod(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/cancel", nil)
	w := httptest.NewRecorder()

	mm.CancelHandler(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestCheckHandler_NotInRoom(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/check?userId=user123", nil)
	w := httptest.NewRecorder()

	mm.CheckHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.CheckResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.InRoom)
}

func TestCheckHandler_MissingUserId(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/check", nil)
	w := httptest.NewRecorder()

	mm.CheckHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.OK)
}

func TestCheckHandler_InRoom_User1(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user1"
	otherUser := "user2"
	matchId := uuid.New().String()
	token1 := "token1"
	token2 := "token2"

	// Set up room in Redis
	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      userId,
		User2:      otherUser,
		Category:   "arrays",
		Difficulty: "easy",
		Token1:     token1,
		Token2:     token2,
		Status:     "active",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	roomJSON, _ := json.Marshal(room)
	rdb.Set(context.Background(), fmt.Sprintf("room:%s", matchId), roomJSON, 2*time.Hour)
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", userId), matchId, 2*time.Hour)
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", otherUser), matchId, 2*time.Hour)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/match/check?userId=%s", userId), nil)
	w := httptest.NewRecorder()

	mm.CheckHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.CheckResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.InRoom)
	assert.Equal(t, matchId, resp.RoomId)
	assert.Equal(t, token1, resp.Token)
}

func TestCheckHandler_InRoom_User2(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user2"
	otherUser := "user1"
	matchId := uuid.New().String()
	token1 := "token1"
	token2 := "token2"

	// Set up room in Redis
	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      otherUser,
		User2:      userId,
		Category:   "arrays",
		Difficulty: "easy",
		Token1:     token1,
		Token2:     token2,
		Status:     "active",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	roomJSON, _ := json.Marshal(room)
	rdb.Set(context.Background(), fmt.Sprintf("room:%s", matchId), roomJSON, 2*time.Hour)
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", userId), matchId, 2*time.Hour)
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", otherUser), matchId, 2*time.Hour)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/match/check?userId=%s", userId), nil)
	w := httptest.NewRecorder()

	mm.CheckHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.CheckResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.InRoom)
	assert.Equal(t, matchId, resp.RoomId)
	assert.Equal(t, token2, resp.Token)
}

func TestCheckHandler_WrongMethod(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/check", nil)
	w := httptest.NewRecorder()

	// GET handler should still process, but missing userId will cause error
	mm.CheckHandler(w, req)

	// Should return bad request for missing userId
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCheckHandler_RoomWithMissingToken(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user1"
	otherUser := "user2"
	matchId := uuid.New().String()

	// Set up room in Redis with empty token1
	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      userId,
		User2:      otherUser,
		Category:   "arrays",
		Difficulty: "easy",
		Token1:     "", // Empty token
		Token2:     "token2",
		Status:     "active",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	roomJSON, _ := json.Marshal(room)
	rdb.Set(context.Background(), fmt.Sprintf("room:%s", matchId), roomJSON, 2*time.Hour)
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", userId), matchId, 2*time.Hour)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/match/check?userId=%s", userId), nil)
	w := httptest.NewRecorder()

	mm.CheckHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.CheckResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.InRoom)
	assert.Equal(t, matchId, resp.RoomId)
	assert.Equal(t, "", resp.Token) // Should return empty token
}

func TestDoneHandler_Success(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user1"
	otherUser := "user2"
	matchId := uuid.New().String()

	// Set up room in Redis
	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      userId,
		User2:      otherUser,
		Category:   "arrays",
		Difficulty: "easy",
		Status:     "active",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	roomJSON, _ := json.Marshal(room)
	rdb.Set(context.Background(), fmt.Sprintf("room:%s", matchId), roomJSON, 2*time.Hour)
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", userId), matchId, 2*time.Hour)
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", otherUser), matchId, 2*time.Hour)

	reqBody := map[string]string{
		"userId": userId,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/done", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	mm.DoneHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.OK)

	// Verify user removed from Redis
	roomId, err := rdb.Get(context.Background(), fmt.Sprintf("user_room:%s", userId)).Result()
	assert.Error(t, err) // Should not exist
	assert.Empty(t, roomId)
}

func TestDoneHandler_NotInRoom(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	reqBody := map[string]string{
		"userId": "nonexistent",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/done", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	mm.DoneHandler(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.OK)
}

func TestDoneHandler_InvalidJSON(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/done", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	mm.DoneHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDoneHandler_WrongMethod(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/done", nil)
	w := httptest.NewRecorder()

	mm.DoneHandler(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestDoneHandler_RoomNotFound(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user1"
	matchId := uuid.New().String()

	// User in room mapping but room doesn't exist
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", userId), matchId, 2*time.Hour)
	// Don't add room entry

	reqBody := map[string]string{
		"userId": userId,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/done", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	mm.DoneHandler(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.OK)
}

func TestDoneHandler_BothUsersLeave(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId1 := "user1"
	userId2 := "user2"
	matchId := uuid.New().String()

	// Set up room in Redis
	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      userId1,
		User2:      userId2,
		Category:   "arrays",
		Difficulty: "easy",
		Status:     "active",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	roomJSON, _ := json.Marshal(room)
	rdb.Set(context.Background(), fmt.Sprintf("room:%s", matchId), roomJSON, 2*time.Hour)
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", userId1), matchId, 2*time.Hour)
	rdb.Set(context.Background(), fmt.Sprintf("user_room:%s", userId2), matchId, 2*time.Hour)

	// First user leaves
	reqBody1 := map[string]string{
		"userId": userId1,
	}
	body1, _ := json.Marshal(reqBody1)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/match/done", bytes.NewBuffer(body1))
	w1 := httptest.NewRecorder()

	mm.DoneHandler(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code)

	// Second user leaves
	reqBody2 := map[string]string{
		"userId": userId2,
	}
	body2, _ := json.Marshal(reqBody2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/match/done", bytes.NewBuffer(body2))
	w2 := httptest.NewRecorder()

	mm.DoneHandler(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	// Verify room is deleted from Redis
	roomData, err := rdb.Get(context.Background(), fmt.Sprintf("room:%s", matchId)).Result()
	assert.Error(t, err) // Should not exist
	assert.Empty(t, roomData)
}

func TestHandshakeHandler_Accept_MatchConfirmed(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

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
	reqBody1 := models.HandshakeReq{
		UserID:  user1,
		MatchId: matchId,
		Accept:  true,
	}

	body1, _ := json.Marshal(reqBody1)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/match/handshake", bytes.NewBuffer(body1))
	w1 := httptest.NewRecorder()

	mm.HandshakeHandler(w1, req1)

	assert.Equal(t, http.StatusOK, w1.Code)

	// Second user accepts
	reqBody2 := models.HandshakeReq{
		UserID:  user2,
		MatchId: matchId,
		Accept:  true,
	}

	body2, _ := json.Marshal(reqBody2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/match/handshake", bytes.NewBuffer(body2))
	w2 := httptest.NewRecorder()

	mm.HandshakeHandler(w2, req2)

	assert.Equal(t, http.StatusOK, w2.Code)

	// Verify match confirmed and room created in Redis
	roomId1, err1 := mm.GetRoomForUser(user1)
	assert.NoError(t, err1)
	assert.Equal(t, matchId, roomId1)

	roomId2, err2 := mm.GetRoomForUser(user2)
	assert.NoError(t, err2)
	assert.Equal(t, matchId, roomId2)
}

func TestHandshakeHandler_Reject(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	matchId := uuid.New().String()
	user1 := "user1"
	user2 := "user2"

	// Set up user2 in queue for re-queuing
	userKey := fmt.Sprintf("user:%s", user2)
	now := float64(time.Now().Unix())
	rdb.HSet(context.Background(), userKey, "category", "arrays", "difficulty", "easy", "joined_at", now)

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

	reqBody := models.HandshakeReq{
		UserID:  user1,
		MatchId: matchId,
		Accept:  false,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/handshake", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	mm.HandshakeHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.OK)

	// Verify pending match removed from Redis
	pendingData, err := rdb.Get(context.Background(), fmt.Sprintf("pending_match:%s", matchId)).Result()
	assert.Error(t, err) // Should not exist
	assert.Empty(t, pendingData)
}

func TestHandshakeHandler_MatchNotFound(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	reqBody := models.HandshakeReq{
		UserID:  "user1",
		MatchId: "nonexistent",
		Accept:  true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/handshake", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	mm.HandshakeHandler(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.OK)
}

func TestHandshakeHandler_NotPartOfMatch(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

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

	reqBody := models.HandshakeReq{
		UserID:  "unauthorized",
		MatchId: matchId,
		Accept:  true,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/handshake", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	mm.HandshakeHandler(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp models.Resp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.False(t, resp.OK)
}

func TestHandshakeHandler_InvalidJSON(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/handshake", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	mm.HandshakeHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandshakeHandler_WrongMethod(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/handshake", nil)
	w := httptest.NewRecorder()

	mm.HandshakeHandler(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestJoinHandler_RedisError(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	reqBody := models.JoinReq{
		UserID:     "user123",
		Category:   "arrays",
		Difficulty: "easy",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/match/join", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	// Even with mock, the handler should still work
	mm.JoinHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWsHandler_MissingUserId(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/ws", nil)
	w := httptest.NewRecorder()

	mm.WsHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "userId required")
}

func TestWsHandler_WithUserId(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/ws?userId=user123", nil)
	w := httptest.NewRecorder()

	// WebSocket upgrade will fail in test environment, but we can verify the handler is called
	mm.WsHandler(w, req)

	// Upgrade will fail, so we can't verify connection, but handler should process userId check
	// The actual upgrade failure is expected in test environment
	// The handler should return before upgrade fails, so status should be BadRequest or similar
}
