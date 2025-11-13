package match_management

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"match/internal/models"
	"match/internal/utils"
)

// --- WebSocket Handler ---
func (mm *MatchManager) WsHandler(w http.ResponseWriter, r *http.Request) {
	utils.EnableCORS(w)

	userId := r.URL.Query().Get("userId")
	if userId == "" {
		http.Error(w, "userId required", http.StatusBadRequest)
		return
	}

	conn, err := mm.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	// Register connection to this instance
	mm.RegisterConnection(userId, conn)
	log.Printf("[Instance %s] WebSocket connected for user: %s", mm.instanceID, userId)

	// Keep connection alive and handle disconnection
	for {
		if _, _, err := conn.NextReader(); err != nil {
			mm.UnregisterConnection(userId)
			conn.Close()
			log.Printf("[Instance %s] User %s disconnected", mm.instanceID, userId)
			break
		}
	}
}

// --- Join Handler ---
func (mm *MatchManager) JoinHandler(w http.ResponseWriter, r *http.Request) {
	utils.EnableCORS(w)
	log.Printf("[Instance %s] JoinHandler called", mm.instanceID)

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

	// Check if user is already in a room (from Redis)
	roomId, err := mm.GetRoomForUser(req.UserID)
	if err == nil && roomId != "" {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{OK: false, Info: "already in a room"})
		return
	}

	// Add user to queues
	now := float64(time.Now().Unix())
	userKey := fmt.Sprintf("user:%s", req.UserID)

	// Track join info with original preferences
	if err := mm.rdb.HSet(mm.ctx, userKey, map[string]interface{}{
		"category":   req.Category,
		"difficulty": req.Difficulty,
		"joined_at":  now,
		"stage":      1,
	}).Err(); err != nil {
		log.Printf("[Instance %s] Failed to set user data in Redis: %v", mm.instanceID, err)
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{OK: false, Info: "failed to join queue"})
		return
	}

	// Add to queues
	if err := mm.rdb.ZAdd(mm.ctx, fmt.Sprintf("queue:%s:%s", req.Category, req.Difficulty), redis.Z{Score: now, Member: req.UserID}).Err(); err != nil {
		log.Printf("[Instance %s] Failed to add to exact queue: %v", mm.instanceID, err)
	}
	if err := mm.rdb.ZAdd(mm.ctx, fmt.Sprintf("queue:%s", req.Category), redis.Z{Score: now, Member: req.UserID}).Err(); err != nil {
		log.Printf("[Instance %s] Failed to add to category queue: %v", mm.instanceID, err)
	}
	if err := mm.rdb.ZAdd(mm.ctx, "queue:all", redis.Z{Score: now, Member: req.UserID}).Err(); err != nil {
		log.Printf("[Instance %s] Failed to add to all queue: %v", mm.instanceID, err)
	}

	log.Printf("[Instance %s] User %s joined queue: category=%s, difficulty=%s", mm.instanceID, req.UserID, req.Category, req.Difficulty)

	// Try immediate match
	mm.tryMatchStage(req.Category, req.Difficulty, 1)

	utils.WriteJSON(w, http.StatusOK, models.Resp{OK: true, Info: "queued"})
}

// --- Cancel Handler ---
func (mm *MatchManager) CancelHandler(w http.ResponseWriter, r *http.Request) {
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
	user, err := mm.rdb.HGetAll(mm.ctx, userKey).Result()
	if err != nil || len(user) == 0 {
		utils.WriteJSON(w, http.StatusNotFound, models.Resp{OK: false, Info: "not in queue"})
		return
	}

	category := user["category"]
	difficulty := user["difficulty"]

	// Remove from all queues
	mm.removeUser(req.UserID, category, difficulty)

	log.Printf("[Instance %s] User %s cancelled matchmaking", mm.instanceID, req.UserID)
	utils.WriteJSON(w, http.StatusOK, models.Resp{OK: true, Info: "cancelled"})
}

// --- Check Handler ---
func (mm *MatchManager) CheckHandler(w http.ResponseWriter, r *http.Request) {
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

	// Check Redis for room assignment
	roomId, err := mm.GetRoomForUser(userId)
	if err != nil || roomId == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(models.CheckResp{InRoom: false})
		return
	}

	// Get room info from Redis
	room, err := mm.GetRoomInfo(roomId)
	if err != nil {
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
func (mm *MatchManager) DoneHandler(w http.ResponseWriter, r *http.Request) {
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

	// Get room from Redis
	roomId, err := mm.GetRoomForUser(req.UserID)
	if err != nil || roomId == "" {
		utils.WriteJSON(w, http.StatusNotFound, models.Resp{OK: false, Info: "not in a room"})
		return
	}

	// Get room info
	room, err := mm.GetRoomInfo(roomId)
	if err != nil {
		utils.WriteJSON(w, http.StatusNotFound, models.Resp{OK: false, Info: "room not found"})
		return
	}

	// Remove this user's room assignment
	mm.rdb.Del(mm.ctx, fmt.Sprintf("user_room:%s", req.UserID))

	// Determine other user
	otherUser := room.User1
	if req.UserID == room.User1 {
		otherUser = room.User2
	}

	// Check if other user is still in room
	otherRoomId, err := mm.GetRoomForUser(otherUser)
	otherStillInRoom := err == nil && otherRoomId == roomId

	if otherStillInRoom {
		log.Printf("[Instance %s] User %s left room %s, partner %s still in room", mm.instanceID, req.UserID, roomId, otherUser)
		mm.sendToUser(otherUser, map[string]interface{}{
			"type":    "partner_left",
			"message": "Your partner has left the room",
			"roomId":  roomId,
		})
	} else {
		// Both users are done, delete room from Redis
		mm.rdb.Del(mm.ctx, fmt.Sprintf("room:%s", roomId))
		log.Printf("[Instance %s] Room %s deleted - both users done", mm.instanceID, roomId)
	}

	utils.WriteJSON(w, http.StatusOK, models.Resp{OK: true, Info: "left room"})
}

// --- Handshake Handler ---
func (mm *MatchManager) HandshakeHandler(w http.ResponseWriter, r *http.Request) {
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

	// Get pending match from Redis
	pendingKey := fmt.Sprintf("pending_match:%s", req.MatchId)
	pendingJSON, err := mm.rdb.Get(mm.ctx, pendingKey).Result()
	if err != nil {
		utils.WriteJSON(w, http.StatusNotFound, models.Resp{OK: false, Info: "match not found or expired"})
		return
	}

	var pending models.PendingMatch
	if err := json.Unmarshal([]byte(pendingJSON), &pending); err != nil {
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{OK: false, Info: "failed to parse match"})
		return
	}

	// Verify user is part of this match
	if req.UserID != pending.User1 && req.UserID != pending.User2 {
		utils.WriteJSON(w, http.StatusForbidden, models.Resp{OK: false, Info: "not part of this match"})
		return
	}

	if !req.Accept {
		// User rejected the match
		log.Printf("[Instance %s] User %s rejected match %s", mm.instanceID, req.UserID, req.MatchId)

		// Determine the other user
		otherUser := pending.User1
		otherUserCat := pending.User1Cat
		otherUserDiff := pending.User1Diff
		if req.UserID == pending.User1 {
			otherUser = pending.User2
			otherUserCat = pending.User2Cat
			otherUserDiff = pending.User2Diff
		}

		// Re-queue the other user
		mm.requeueUser(otherUser, otherUserCat, otherUserDiff)

		// Notify other user they were re-queued
		mm.sendToUser(otherUser, map[string]interface{}{
			"type":    "requeued",
			"message": "Other user declined the match. You have been re-queued.",
		})

		// Remove rejecting user from queue completely
		rejectingUserCat := pending.User1Cat
		rejectingUserDiff := pending.User1Diff
		if req.UserID == pending.User2 {
			rejectingUserCat = pending.User2Cat
			rejectingUserDiff = pending.User2Diff
		}
		mm.removeUser(req.UserID, rejectingUserCat, rejectingUserDiff)

		// Clean up pending match in Redis
		mm.rdb.Del(mm.ctx, pendingKey)
		mm.rdb.Del(mm.ctx, fmt.Sprintf("handshake:%s:%s", req.MatchId, pending.User1))
		mm.rdb.Del(mm.ctx, fmt.Sprintf("handshake:%s:%s", req.MatchId, pending.User2))

		utils.WriteJSON(w, http.StatusOK, models.Resp{OK: true, Info: "match declined"})
		return
	}

	// User accepted - use the HandleMatchAccept method
	err = mm.HandleMatchAccept(req.MatchId, req.UserID)
	if err != nil {
		log.Printf("[Instance %s] Failed to handle match accept: %v", mm.instanceID, err)
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{OK: false, Info: "failed to process acceptance"})
		return
	}

	utils.WriteJSON(w, http.StatusOK, models.Resp{OK: true, Info: "handshake received"})
}

// --- Session Feedback Handler ---
// Receives session metrics after a session ends and updates Elo ratings
func (mm *MatchManager) SessionFeedbackHandler(w http.ResponseWriter, r *http.Request) {
	utils.EnableCORS(w)
	log.Printf("[Instance %s] SessionFeedbackHandler called", mm.instanceID)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var metrics models.SessionMetrics
	if err := json.NewDecoder(r.Body).Decode(&metrics); err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{OK: false, Info: "invalid json"})
		return
	}

	// Validate required fields
	if metrics.User1ID == "" || metrics.User2ID == "" {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{OK: false, Info: "user IDs required"})
		return
	}

	if metrics.SessionDuration <= 0 {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{OK: false, Info: "invalid session duration"})
		return
	}

	log.Printf("[Instance %s] Processing session feedback: User1=%s, User2=%s, Duration=%ds",
		mm.instanceID, metrics.User1ID, metrics.User2ID, metrics.SessionDuration)

	// Process metrics and update Elo ratings
	eloUpdates, err := mm.eloManager.ProcessSessionMetrics(&metrics)
	if err != nil {
		log.Printf("[Instance %s] Failed to process session metrics: %v", mm.instanceID, err)
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{OK: false, Info: "failed to update Elo ratings"})
		return
	}

	log.Printf("[Instance %s] Successfully updated Elo ratings for %d users", mm.instanceID, len(eloUpdates))

	utils.WriteJSON(w, http.StatusOK, models.Resp{
		OK:   true,
		Info: eloUpdates,
	})
}
