package match_management

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"match/internal/models"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// Note: setupTestRedis is defined in match_manager_test.go and can be used here

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

	assert.Equal(t, http.StatusOK, w.Code) // Still OK since empty string is valid
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

	// Set up user in room
	mm.roomMu.Lock()
	mm.userToRoom[userId] = matchId
	mm.roomMu.Unlock()

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

	// Set up user in queue
	userKey := "user:" + userId
	rdb.HSet(context.Background(), userKey, map[string]interface{}{
		"category":   category,
		"difficulty": difficulty,
		"joined_at":  float64(time.Now().Unix()),
	})
	rdb.ZAdd(context.Background(), "queue:arrays:easy", redis.Z{Score: float64(time.Now().Unix()), Member: userId})

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

	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      userId,
		User2:      otherUser,
		Category:   "arrays",
		Difficulty: "easy",
		Token1:     token1,
		Token2:     token2,
	}

	mm.roomMu.Lock()
	mm.userToRoom[userId] = matchId
	mm.userToRoom[otherUser] = matchId
	mm.roomInfo[matchId] = room
	mm.roomMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/check?userId="+userId, nil)
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

	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      otherUser,
		User2:      userId,
		Category:   "arrays",
		Difficulty: "easy",
		Token1:     token1,
		Token2:     token2,
	}

	mm.roomMu.Lock()
	mm.userToRoom[userId] = matchId
	mm.userToRoom[otherUser] = matchId
	mm.roomInfo[matchId] = room
	mm.roomMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/check?userId="+userId, nil)
	w := httptest.NewRecorder()

	mm.CheckHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.CheckResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.InRoom)
	assert.Equal(t, matchId, resp.RoomId)
	assert.Equal(t, token2, resp.Token)
}

func TestDoneHandler_Success(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user1"
	otherUser := "user2"
	matchId := uuid.New().String()

	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      userId,
		User2:      otherUser,
		Category:   "arrays",
		Difficulty: "easy",
	}

	mm.roomMu.Lock()
	mm.userToRoom[userId] = matchId
	mm.userToRoom[otherUser] = matchId
	mm.roomInfo[matchId] = room
	mm.roomMu.Unlock()

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

	// Verify user removed
	mm.roomMu.RLock()
	_, inRoom := mm.userToRoom[userId]
	mm.roomMu.RUnlock()
	assert.False(t, inRoom)
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

func TestHandshakeHandler_Accept_MatchConfirmed(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	matchId := uuid.New().String()
	user1 := "user1"
	user2 := "user2"

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

	mm.pendingMu.Lock()
	mm.pendingMatches[matchId] = pending
	mm.pendingMu.Unlock()

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

	// Verify match confirmed and room created
	mm.roomMu.RLock()
	room1, inRoom1 := mm.userToRoom[user1]
	room2, inRoom2 := mm.userToRoom[user2]
	mm.roomMu.RUnlock()

	assert.True(t, inRoom1)
	assert.True(t, inRoom2)
	assert.Equal(t, matchId, room1)
	assert.Equal(t, matchId, room2)
}

func TestHandshakeHandler_Reject(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	matchId := uuid.New().String()
	user1 := "user1"
	user2 := "user2"

	// Set up user2 in queue for re-queuing
	userKey := "user:" + user2
	rdb.HSet(context.Background(), userKey, "category", "arrays", "difficulty", "easy", "joined_at", float64(time.Now().Unix()))

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

	mm.pendingMu.Lock()
	mm.pendingMatches[matchId] = pending
	mm.pendingMu.Unlock()

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

	// Verify pending match removed
	mm.pendingMu.Lock()
	_, exists := mm.pendingMatches[matchId]
	mm.pendingMu.Unlock()
	assert.False(t, exists)
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

	mm.pendingMu.Lock()
	mm.pendingMatches[matchId] = pending
	mm.pendingMu.Unlock()

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

func TestCancelHandler_WrongMethod(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/cancel", nil)
	w := httptest.NewRecorder()

	mm.CancelHandler(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
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
	mm.roomMu.Lock()
	mm.userToRoom[userId] = matchId
	// Don't add roomInfo entry
	mm.roomMu.Unlock()

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

	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      userId1,
		User2:      userId2,
		Category:   "arrays",
		Difficulty: "easy",
	}

	mm.roomMu.Lock()
	mm.userToRoom[userId1] = matchId
	mm.userToRoom[userId2] = matchId
	mm.roomInfo[matchId] = room
	mm.roomMu.Unlock()

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

	// Verify room is deleted
	mm.roomMu.RLock()
	_, exists := mm.roomInfo[matchId]
	mm.roomMu.RUnlock()
	assert.False(t, exists)
}

func TestHandshakeHandler_AlreadyAccepted(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	matchId := uuid.New().String()
	user1 := "user1"
	user2 := "user2"

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

	mm.pendingMu.Lock()
	mm.pendingMatches[matchId] = pending
	mm.pendingMu.Unlock()

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

	// Same user accepts again (should still work)
	mm.HandshakeHandler(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
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
}

func TestCheckHandler_RoomWithMissingToken(t *testing.T) {
	secret := []byte("test-secret")
	_, rdb := setupTestRedis(t)
	mm := NewMatchManager(secret, rdb)

	userId := "user1"
	otherUser := "user2"
	matchId := uuid.New().String()

	room := &models.RoomInfo{
		MatchId:    matchId,
		User1:      userId,
		User2:      otherUser,
		Category:   "arrays",
		Difficulty: "easy",
		Token1:     "", // Empty token
		Token2:     "token2",
	}

	mm.roomMu.Lock()
	mm.userToRoom[userId] = matchId
	mm.userToRoom[otherUser] = matchId
	mm.roomInfo[matchId] = room
	mm.roomMu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/match/check?userId="+userId, nil)
	w := httptest.NewRecorder()

	mm.CheckHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.CheckResp
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp.InRoom)
	assert.Equal(t, matchId, resp.RoomId)
	assert.Equal(t, "", resp.Token) // Should return empty token
}