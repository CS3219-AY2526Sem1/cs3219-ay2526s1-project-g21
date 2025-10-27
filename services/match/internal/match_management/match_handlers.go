package match_management

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"match/internal/models"
	"match/internal/utils"
)

// --- WebSocket Handler ---
func (matchManager *MatchManager) WsHandler(w http.ResponseWriter, r *http.Request) {
	utils.EnableCORS(w)

	userId := r.URL.Query().Get("userId")
	if userId == "" {
		http.Error(w, "userId required", http.StatusBadRequest)
		return
	}

	conn, err := matchManager.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	matchManager.mu.Lock()
	matchManager.connections[userId] = conn
	matchManager.mu.Unlock()
	log.Printf("WebSocket connected for user: %s", userId)

	for {
		if _, _, err := conn.NextReader(); err != nil {
			matchManager.mu.Lock()
			delete(matchManager.connections, userId)
			matchManager.mu.Unlock()
			conn.Close()
			log.Printf("User %s disconnected", userId)
			break
		}
	}
}

// --- Join Handler ---
func (matchManager *MatchManager) JoinHandler(w http.ResponseWriter, r *http.Request) {
	utils.EnableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.JoinReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{OK: false, Info: "invalid json"})
		return
	}

	// Check if user is already in a room
	matchManager.roomMu.RLock()
	_, inRoom := matchManager.userToRoom[req.UserID]
	matchManager.roomMu.RUnlock()

	if inRoom {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{OK: false, Info: "already in a room"})
		return
	}

	// Add user to queues
	now := float64(time.Now().Unix())
	userKey := fmt.Sprintf("user:%s", req.UserID)

	// Track join info with original preferences
	if err := matchManager.rdb.HSet(matchManager.ctx, userKey, map[string]interface{}{
		"category":   req.Category,
		"difficulty": req.Difficulty,
		"joined_at":  now,
		"stage":      1,
	}).Err(); err != nil {
		log.Printf("Failed to set user data in Redis: %v", err)
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{OK: false, Info: "failed to join queue"})
		return
	}

	// Add to queues
	if err := matchManager.rdb.ZAdd(matchManager.ctx, fmt.Sprintf("queue:%s:%s", req.Category, req.Difficulty), redis.Z{Score: now, Member: req.UserID}).Err(); err != nil {
		log.Printf("Failed to add to exact queue: %v", err)
	}
	if err := matchManager.rdb.ZAdd(matchManager.ctx, fmt.Sprintf("queue:%s", req.Category), redis.Z{Score: now, Member: req.UserID}).Err(); err != nil {
		log.Printf("Failed to add to category queue: %v", err)
	}
	if err := matchManager.rdb.ZAdd(matchManager.ctx, "queue:all", redis.Z{Score: now, Member: req.UserID}).Err(); err != nil {
		log.Printf("Failed to add to all queue: %v", err)
	}

	log.Printf("User %s joined queue: category=%s, difficulty=%s", req.UserID, req.Category, req.Difficulty)

	// Try immediate match
	matchManager.tryMatchStage(req.Category, req.Difficulty, 1)

	utils.WriteJSON(w, http.StatusOK, models.Resp{OK: true, Info: "queued"})
}

// --- Cancel Handler ---
func (matchManager *MatchManager) CancelHandler(w http.ResponseWriter, r *http.Request) {
	utils.EnableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"userId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{OK: false, Info: "invalid json"})
		return
	}

	// Get user's original preferences from Redis
	userKey := fmt.Sprintf("user:%s", req.UserID)
	user, err := matchManager.rdb.HGetAll(matchManager.ctx, userKey).Result()
	if err != nil || len(user) == 0 {
		utils.WriteJSON(w, http.StatusNotFound, models.Resp{OK: false, Info: "not in queue"})
		return
	}

	category := user["category"]
	difficulty := user["difficulty"]

	// Remove from all queues
	matchManager.removeUser(req.UserID, category, difficulty)

	utils.WriteJSON(w, http.StatusOK, models.Resp{OK: true, Info: "cancelled"})
}

// --- Check Handler ---
func (matchManager *MatchManager) CheckHandler(w http.ResponseWriter, r *http.Request) {
	utils.EnableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	userId := r.URL.Query().Get("userId")
	if userId == "" {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{OK: false, Info: "userId required"})
		return
	}

	matchManager.roomMu.RLock()
	roomId, inRoom := matchManager.userToRoom[userId]
	var room *models.RoomInfo
	if inRoom {
		room = matchManager.roomInfo[roomId]
	}
	matchManager.roomMu.RUnlock()

	if !inRoom || room == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(models.CheckResp{InRoom: false})
		return
	}

	// Return appropriate token based on which user is querying
	token := room.Token1
	if userId == room.User2 {
		token = room.Token2
	}

	resp := models.CheckResp{
		InRoom:     true,
		RoomId:     room.MatchId,
		Category:   room.Category,
		Difficulty: room.Difficulty,
		Token:      token,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// --- Done Handler ---
func (matchManager *MatchManager) DoneHandler(w http.ResponseWriter, r *http.Request) {
	utils.EnableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{OK: false, Info: "invalid json"})
		return
	}

	matchManager.roomMu.Lock()
	defer matchManager.roomMu.Unlock()

	roomId, inRoom := matchManager.userToRoom[req.UserID]
	if !inRoom {
		utils.WriteJSON(w, http.StatusNotFound, models.Resp{OK: false, Info: "not in a room"})
		return
	}

	room := matchManager.roomInfo[roomId]
	if room == nil {
		utils.WriteJSON(w, http.StatusNotFound, models.Resp{OK: false, Info: "room not found"})
		return
	}

	// Remove user from room
	delete(matchManager.userToRoom, req.UserID)

	// Check if other user is still in room
	otherUser := room.User1
	if req.UserID == room.User1 {
		otherUser = room.User2
	}

	_, otherStillInRoom := matchManager.userToRoom[otherUser]
	if otherStillInRoom {
		data, _ := json.Marshal(room)
		matchManager.rdb.Publish(matchManager.ctx, "matches", data)
		// Notify the partner that this user left
		log.Printf("User %s left room %s, notifying partner %s", req.UserID, roomId, otherUser)
		matchManager.sendToUser(otherUser, map[string]interface{}{
			"type":    "partner_left",
			"message": "Your partner has left the room",
			"roomId":  roomId,
		})
	} else {
		// Both users are done, delete room
		delete(matchManager.roomInfo, roomId)
		log.Printf("Room %s deleted - both users done", roomId)
	}

	utils.WriteJSON(w, http.StatusOK, models.Resp{OK: true, Info: "left room"})
}

// --- Handshake Handler ---
func (matchManager *MatchManager) HandshakeHandler(w http.ResponseWriter, r *http.Request) {
	utils.EnableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.HandshakeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{OK: false, Info: "invalid json"})
		return
	}

	matchManager.pendingMu.Lock()
	pending, exists := matchManager.pendingMatches[req.MatchId]
	matchManager.pendingMu.Unlock()

	if !exists {
		utils.WriteJSON(w, http.StatusNotFound, models.Resp{OK: false, Info: "match not found or expired"})
		return
	}

	// Verify user is part of this match
	if req.UserID != pending.User1 && req.UserID != pending.User2 {
		utils.WriteJSON(w, http.StatusForbidden, models.Resp{OK: false, Info: "not part of this match"})
		return
	}

	matchManager.pendingMu.Lock()

	if !req.Accept {
		// User rejected the match
		log.Printf("User %s rejected match %s", req.UserID, req.MatchId)

		// Re-queue the other user
		otherUser := pending.User1
		otherUserCat := pending.User1Cat
		otherUserDiff := pending.User1Diff
		if req.UserID == pending.User1 {
			otherUser = pending.User2
			otherUserCat = pending.User2Cat
			otherUserDiff = pending.User2Diff
		}

		// Get original join time
		userKey := fmt.Sprintf("user:%s", otherUser)
		userData, _ := matchManager.rdb.HGetAll(matchManager.ctx, userKey).Result()
		originalTime := time.Now().Unix()
		if joinedAt, ok := userData["joined_at"]; ok {
			if jt, err := strconv.ParseFloat(joinedAt, 64); err == nil {
				originalTime = int64(jt)
			}
		}

		// Re-add other user to queue with original priority
		matchManager.rdb.HSet(matchManager.ctx, userKey, map[string]interface{}{
			"category":   otherUserCat,
			"difficulty": otherUserDiff,
			"joined_at":  float64(originalTime),
			"stage":      1,
		})
		matchManager.rdb.ZAdd(matchManager.ctx, fmt.Sprintf("queue:%s:%s", otherUserCat, otherUserDiff), redis.Z{Score: float64(originalTime), Member: otherUser})
		matchManager.rdb.ZAdd(matchManager.ctx, fmt.Sprintf("queue:%s", otherUserCat), redis.Z{Score: float64(originalTime), Member: otherUser})
		matchManager.rdb.ZAdd(matchManager.ctx, "queue:all", redis.Z{Score: float64(originalTime), Member: otherUser})

		// Notify other user they were re-queued
		matchManager.sendToUser(otherUser, map[string]interface{}{
			"type":    "requeued",
			"message": "Other user declined the match. You have been re-queued.",
		})

		// Remove rejecting user from queue completely
		matchManager.removeUser(req.UserID, pending.User1Cat, pending.User1Diff)
		if req.UserID == pending.User2 {
			matchManager.removeUser(req.UserID, pending.User2Cat, pending.User2Diff)
		}

		delete(matchManager.pendingMatches, req.MatchId)
		matchManager.pendingMu.Unlock()

		utils.WriteJSON(w, http.StatusOK, models.Resp{OK: true, Info: "match declined"})
		return
	}

	// User accepted
	pending.Handshakes[req.UserID] = true

	// Check if both accepted
	if len(pending.Handshakes) == 2 {
		// Both accepted - confirm match
		log.Printf("Match %s confirmed by both users", req.MatchId)

		room := &models.RoomInfo{
			MatchId:    pending.MatchId,
			User1:      pending.User1,
			User2:      pending.User2,
			Category:   pending.Category,
			Difficulty: pending.Difficulty,
			Status:     "active",
			Token1:     pending.Token1,
			Token2:     pending.Token2,
			CreatedAt:  pending.CreatedAt.Format(time.RFC3339),
		}

		// Store in room maps
		matchManager.roomMu.Lock()
		matchManager.userToRoom[pending.User1] = pending.MatchId
		matchManager.userToRoom[pending.User2] = pending.MatchId
		matchManager.roomInfo[pending.MatchId] = room
		matchManager.roomMu.Unlock()

		// Notify both users
		matchManager.sendToUser(pending.User1, map[string]interface{}{
			"type":       "match_confirmed",
			"matchId":    pending.MatchId,
			"category":   pending.Category,
			"difficulty": pending.Difficulty,
			"token":      pending.Token1,
		})

		matchManager.sendToUser(pending.User2, map[string]interface{}{
			"type":       "match_confirmed",
			"matchId":    pending.MatchId,
			"category":   pending.Category,
			"difficulty": pending.Difficulty,
			"token":      pending.Token2,
		})

		// Clean up
		delete(matchManager.pendingMatches, req.MatchId)
		matchManager.rdb.Del(matchManager.ctx, fmt.Sprintf("user:%s", pending.User1))
		matchManager.rdb.Del(matchManager.ctx, fmt.Sprintf("user:%s", pending.User2))

		// Store match info in Redis
		matchKey := "match:" + req.MatchId
		matchManager.rdb.HSet(matchManager.ctx, matchKey, map[string]interface{}{
			"id":         req.MatchId,
			"user1":      pending.User1,
			"user2":      pending.User1,
			"category":   pending.Category,
			"difficulty": pending.Difficulty,
			"token1":     pending.Token1,
			"token2":     pending.Token1,
			"created_at": time.Now().Format(time.RFC3339),
			"status":     "pending", // Room creation pending
		})
		matchManager.rdb.LPush(matchManager.ctx, "matches", req.MatchId)

		// Publish match event to Redis for collab service to process
		data, _ := json.Marshal(room)
		matchManager.rdb.Publish(matchManager.ctx, "matches", data)
	}

	matchManager.pendingMu.Unlock()

	utils.WriteJSON(w, http.StatusOK, models.Resp{OK: true, Info: "handshake received"})
}
