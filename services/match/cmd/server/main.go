package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var (
	ctx         = context.Background()
	rdb         *redis.Client
	upgrader    = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	connections = make(map[string]*websocket.Conn)
	mu          sync.Mutex
	jwtSecret   []byte
	
	// Maps for room tracking
	userToRoom = make(map[string]string)
	roomInfo   = make(map[string]*RoomInfo)
	roomMu     sync.RWMutex
	
	// Pending matches awaiting handshake
	pendingMatches = make(map[string]*PendingMatch)
	pendingMu      sync.Mutex
)

// Difficulty levels
const (
	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"
)

// --- Models ---
type JoinReq struct {
	UserID     string `json:"userId"`
	Category   string `json:"category"`
	Difficulty string `json:"difficulty"`
}

type Resp struct {
	OK   bool        `json:"ok"`
	Info interface{} `json:"info"`
}

type RoomInfo struct {
	MatchId    string `json:"matchId"`
	User1      string `json:"user1"`
	User2      string `json:"user2"`
	Category   string `json:"category"`
	Difficulty string `json:"difficulty"`
	Status     string `json:"status"`
	Token1     string `json:"token1"`
	Token2     string `json:"token2"`
	CreatedAt  string `json:"createdAt"`
}

type PendingMatch struct {
	MatchId      string
	User1        string
	User2        string
	Category     string
	Difficulty   string
	User1Cat     string
	User1Diff    string
	User2Cat     string
	User2Diff    string
	Token1       string
	Token2       string
	Handshakes   map[string]bool
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

type HandshakeReq struct {
	UserID  string `json:"userId"`
	MatchId string `json:"matchId"`
	Accept  bool   `json:"accept"`
}

type CheckResp struct {
	InRoom     bool   `json:"inRoom"`
	RoomId     string `json:"roomId,omitempty"`
	Category   string `json:"category,omitempty"`
	Difficulty string `json:"difficulty,omitempty"`
	Token      string `json:"token,omitempty"`
}

// --- JWT Helper ---
func generateRoomToken(matchId, userId string) (string, error) {
	claims := jwt.MapClaims{
		"matchId": matchId,
		"userId":  userId,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// --- Helper Functions ---
func writeJSON(w http.ResponseWriter, code int, resp Resp) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func difficultyToInt(diff string) int {
	switch diff {
	case DifficultyEasy:
		return 1
	case DifficultyMedium:
		return 2
	case DifficultyHard:
		return 3
	default:
		return 2
	}
}

func intToDifficulty(val int) string {
	switch val {
	case 1:
		return DifficultyEasy
	case 2:
		return DifficultyMedium
	case 3:
		return DifficultyHard
	default:
		return DifficultyMedium
	}
}

func averageDifficulty(diff1, diff2 string) string {
	d1 := difficultyToInt(diff1)
	d2 := difficultyToInt(diff2)
	avg := int(math.Floor(float64(d1+d2) / 2.0))
	return intToDifficulty(avg)
}

func randomCategory(cat1, cat2 string) string {
	if rand.Intn(2) == 0 {
		return cat1
	}
	return cat2
}

// --- WebSocket Handler ---
func wsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	userId := r.URL.Query().Get("userId")
	if userId == "" {
		http.Error(w, "userId required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	mu.Lock()
	connections[userId] = conn
	mu.Unlock()
	log.Printf("WebSocket connected for user: %s", userId)

	for {
		if _, _, err := conn.NextReader(); err != nil {
			mu.Lock()
			delete(connections, userId)
			mu.Unlock()
			conn.Close()
			log.Printf("User %s disconnected", userId)
			break
		}
	}
}

// --- Send message to specific user via WebSocket ---
func sendToUser(userId string, data interface{}) {
	mu.Lock()
	conn, ok := connections[userId]
	mu.Unlock()

	if ok {
		if err := conn.WriteJSON(data); err != nil {
			log.Printf("Error sending to user %s: %v", userId, err)
			mu.Lock()
			delete(connections, userId)
			mu.Unlock()
			conn.Close()
		}
	}
}

// --- Redis Subscriber ---
func subscribeToRedis() {
	subscriber := rdb.Subscribe(ctx, "matches")
	ch := subscriber.Channel()

	for msg := range ch {
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Println("Failed to parse event:", err)
			continue
		}
		log.Printf("Redis event received: %v", event)
	}
}

// --- Join Handler ---
func joinHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req JoinReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Info: "invalid json"})
		return
	}

	// Check if user is already in a room
	roomMu.RLock()
	_, inRoom := userToRoom[req.UserID]
	roomMu.RUnlock()

	if inRoom {
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Info: "already in a room"})
		return
	}

	// Add user to queues
	now := float64(time.Now().Unix())
	userKey := fmt.Sprintf("user:%s", req.UserID)

	// Track join info with original preferences
	rdb.HSet(ctx, userKey, map[string]interface{}{
		"category":   req.Category,
		"difficulty": req.Difficulty,
		"joined_at":  now,
		"stage":      1,
	})

	// Add to queues
	rdb.ZAdd(ctx, fmt.Sprintf("queue:%s:%s", req.Category, req.Difficulty), redis.Z{Score: now, Member: req.UserID})
	rdb.ZAdd(ctx, fmt.Sprintf("queue:%s", req.Category), redis.Z{Score: now, Member: req.UserID})
	rdb.ZAdd(ctx, "queue:all", redis.Z{Score: now, Member: req.UserID})

	// Try immediate match
	tryMatchStage(req.Category, req.Difficulty, 1)

	writeJSON(w, http.StatusOK, Resp{OK: true, Info: "queued"})
}

// --- Cancel Handler ---
func cancelHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

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
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Info: "invalid json"})
		return
	}

	// Get user's original preferences from Redis
	userKey := fmt.Sprintf("user:%s", req.UserID)
	user, err := rdb.HGetAll(ctx, userKey).Result()
	if err != nil || len(user) == 0 {
		writeJSON(w, http.StatusNotFound, Resp{OK: false, Info: "not in queue"})
		return
	}

	category := user["category"]
	difficulty := user["difficulty"]

	// Remove from all queues
	removeUser(req.UserID, category, difficulty)

	writeJSON(w, http.StatusOK, Resp{OK: true, Info: "cancelled"})
}

// --- Check Handler ---
func checkHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	userId := r.URL.Query().Get("userId")
	if userId == "" {
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Info: "userId required"})
		return
	}

	roomMu.RLock()
	roomId, inRoom := userToRoom[userId]
	var room *RoomInfo
	if inRoom {
		room = roomInfo[roomId]
	}
	roomMu.RUnlock()

	if !inRoom || room == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(CheckResp{InRoom: false})
		return
	}

	// Return appropriate token based on which user is querying
	token := room.Token1
	if userId == room.User2 {
		token = room.Token2
	}

	resp := CheckResp{
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
func doneHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

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
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Info: "invalid json"})
		return
	}

	roomMu.Lock()
	defer roomMu.Unlock()

	roomId, inRoom := userToRoom[req.UserID]
	if !inRoom {
		writeJSON(w, http.StatusNotFound, Resp{OK: false, Info: "not in a room"})
		return
	}

	room := roomInfo[roomId]
	if room == nil {
		writeJSON(w, http.StatusNotFound, Resp{OK: false, Info: "room not found"})
		return
	}

	// Remove user from room
	delete(userToRoom, req.UserID)

	// Check if other user is still in room
	otherUser := room.User1
	if req.UserID == room.User1 {
		otherUser = room.User2
	}

	_, otherStillInRoom := userToRoom[otherUser]
	if !otherStillInRoom {
		// Both users are done, delete room
		delete(roomInfo, roomId)
		log.Printf("Room %s deleted - both users done", roomId)
	}

	writeJSON(w, http.StatusOK, Resp{OK: true, Info: "left room"})
}

// --- Handshake Handler ---
func handshakeHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HandshakeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, Resp{OK: false, Info: "invalid json"})
		return
	}

	pendingMu.Lock()
	pending, exists := pendingMatches[req.MatchId]
	pendingMu.Unlock()

	if !exists {
		writeJSON(w, http.StatusNotFound, Resp{OK: false, Info: "match not found or expired"})
		return
	}

	// Verify user is part of this match
	if req.UserID != pending.User1 && req.UserID != pending.User2 {
		writeJSON(w, http.StatusForbidden, Resp{OK: false, Info: "not part of this match"})
		return
	}

	pendingMu.Lock()
	
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
		userData, _ := rdb.HGetAll(ctx, userKey).Result()
		originalTime := time.Now().Unix()
		if joinedAt, ok := userData["joined_at"]; ok {
			if jt, err := strconv.ParseFloat(joinedAt, 64); err == nil {
				originalTime = int64(jt)
			}
		}
		
		// Re-add other user to queue with original priority
		rdb.HSet(ctx, userKey, map[string]interface{}{
			"category":   otherUserCat,
			"difficulty": otherUserDiff,
			"joined_at":  float64(originalTime),
			"stage":      1,
		})
		rdb.ZAdd(ctx, fmt.Sprintf("queue:%s:%s", otherUserCat, otherUserDiff), redis.Z{Score: float64(originalTime), Member: otherUser})
		rdb.ZAdd(ctx, fmt.Sprintf("queue:%s", otherUserCat), redis.Z{Score: float64(originalTime), Member: otherUser})
		rdb.ZAdd(ctx, "queue:all", redis.Z{Score: float64(originalTime), Member: otherUser})
		
		// Notify other user they were re-queued
		sendToUser(otherUser, map[string]interface{}{
			"type":    "requeued",
			"message": "Other user declined the match. You have been re-queued.",
		})
		
		// Remove rejecting user from queue completely
		removeUser(req.UserID, pending.User1Cat, pending.User1Diff)
		if req.UserID == pending.User2 {
			removeUser(req.UserID, pending.User2Cat, pending.User2Diff)
		}
		
		delete(pendingMatches, req.MatchId)
		pendingMu.Unlock()
		
		writeJSON(w, http.StatusOK, Resp{OK: true, Info: "match declined"})
		return
	}
	
	// User accepted
	pending.Handshakes[req.UserID] = true
	
	// Check if both accepted
	if len(pending.Handshakes) == 2 {
		// Both accepted - confirm match
		log.Printf("Match %s confirmed by both users", req.MatchId)
		
		room := &RoomInfo{
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
		roomMu.Lock()
		userToRoom[pending.User1] = pending.MatchId
		userToRoom[pending.User2] = pending.MatchId
		roomInfo[pending.MatchId] = room
		roomMu.Unlock()
		
		// Notify both users
		sendToUser(pending.User1, map[string]interface{}{
			"type":       "match_confirmed",
			"matchId":    pending.MatchId,
			"category":   pending.Category,
			"difficulty": pending.Difficulty,
			"token":      pending.Token1,
		})
		
		sendToUser(pending.User2, map[string]interface{}{
			"type":       "match_confirmed",
			"matchId":    pending.MatchId,
			"category":   pending.Category,
			"difficulty": pending.Difficulty,
			"token":      pending.Token2,
		})
		
		// Clean up
		delete(pendingMatches, req.MatchId)
		rdb.Del(ctx, fmt.Sprintf("user:%s", pending.User1))
		rdb.Del(ctx, fmt.Sprintf("user:%s", pending.User2))

		// Store match info in Redis
		matchKey := "match:" + req.MatchId
		rdb.HSet(ctx, matchKey, map[string]interface{}{
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
		rdb.LPush(ctx, "matches", req.MatchId)

		// Publish match event to Redis for collab service to process
		data, _ := json.Marshal(room)
		rdb.Publish(ctx, "matches", data)
	}
	
	pendingMu.Unlock()
	
	writeJSON(w, http.StatusOK, Resp{OK: true, Info: "handshake received"})
}

func matchmakingLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		keys, _ := rdb.Keys(ctx, "user:*").Result()
		for _, key := range keys {
			user, _ := rdb.HGetAll(ctx, key).Result()
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
				if elapsed > 5 {
					rdb.HSet(ctx, key, "stage", 2)
					tryMatchStage(category, difficulty, 2)
				}
			case 2:
				if elapsed > 15 {
					rdb.HSet(ctx, key, "stage", 3)
					tryMatchStage(category, difficulty, 3)
				}
			case 3:
				if elapsed > 540 {
					removeUser(userId, category, difficulty)
					sendToUser(userId, map[string]interface{}{
						"type":    "timeout",
						"message": "Matchmaking timed out",
					})
				}
			}
		}
	}
}

// --- Pending Match Expiration Loop ---
func pendingMatchExpirationLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		
		pendingMu.Lock()
		for matchId, pending := range pendingMatches {
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
						userData, _ := rdb.HGetAll(ctx, userKey).Result()
						if joinedAt, ok := userData["joined_at"]; ok {
							originalTime, _ = strconv.ParseFloat(joinedAt, 64)
						} else {
							originalTime = float64(time.Now().Unix())
						}
						
						// Re-add to queue
						rdb.HSet(ctx, userKey, map[string]interface{}{
							"category":   cat,
							"difficulty": diff,
							"joined_at":  originalTime,
							"stage":      1,
						})
						rdb.ZAdd(ctx, fmt.Sprintf("queue:%s:%s", cat, diff), redis.Z{Score: originalTime, Member: userId})
						rdb.ZAdd(ctx, fmt.Sprintf("queue:%s", cat), redis.Z{Score: originalTime, Member: userId})
						rdb.ZAdd(ctx, "queue:all", redis.Z{Score: originalTime, Member: userId})
						
						sendToUser(userId, map[string]interface{}{
							"type":    "requeued",
							"message": "Other user did not accept in time. You have been re-queued.",
						})
					}
				}
				
				// Remove users who didn't accept
				if !pending.Handshakes[pending.User1] {
					removeUser(pending.User1, pending.User1Cat, pending.User1Diff)
					sendToUser(pending.User1, map[string]interface{}{
						"type":    "timeout",
						"message": "Match expired - you did not accept in time",
					})
				}
				if !pending.Handshakes[pending.User2] {
					removeUser(pending.User2, pending.User2Cat, pending.User2Diff)
					sendToUser(pending.User2, map[string]interface{}{
						"type":    "timeout",
						"message": "Match expired - you did not accept in time",
					})
				}
				
				delete(pendingMatches, matchId)
			}
		}
		pendingMu.Unlock()
	}
}

// --- Try Match at Stage ---
func tryMatchStage(category, difficulty string, stage int) {
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
		users, _ := rdb.ZRange(ctx, queueKey, 0, 1).Result()
		if len(users) < 2 {
			continue
		}

		u1, u2 := users[0], users[1]
		
		// Get their original preferences
		user1Data, _ := rdb.HGetAll(ctx, fmt.Sprintf("user:%s", u1)).Result()
		user2Data, _ := rdb.HGetAll(ctx, fmt.Sprintf("user:%s", u2)).Result()
		
		createPendingMatch(u1, u2,
			user1Data["category"], user1Data["difficulty"],
			user2Data["category"], user2Data["difficulty"], stage)
		return
	}
}

// --- Create Pending Match ---
func createPendingMatch(u1, u2, cat1, diff1, cat2, diff2 string, stage int) {
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
		finalDiff = averageDifficulty(diff1, diff2)
	case 3:
		// Any match - random category, average difficulty
		finalCat = randomCategory(cat1, cat2)
		finalDiff = averageDifficulty(diff1, diff2)
	}

	// Remove users from their respective queues
	rdb.ZRem(ctx, fmt.Sprintf("queue:%s:%s", cat1, diff1), u1)
	rdb.ZRem(ctx, fmt.Sprintf("queue:%s", cat1), u1)
	rdb.ZRem(ctx, "queue:all", u1)
	
	rdb.ZRem(ctx, fmt.Sprintf("queue:%s:%s", cat2, diff2), u2)
	rdb.ZRem(ctx, fmt.Sprintf("queue:%s", cat2), u2)
	rdb.ZRem(ctx, "queue:all", u2)

	matchID := uuid.New().String()
	token1, _ := generateRoomToken(matchID, u1)
	token2, _ := generateRoomToken(matchID, u2)

	pending := &PendingMatch{
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
		ExpiresAt:  time.Now().Add(15 * time.Second),
	}

	pendingMu.Lock()
	pendingMatches[matchID] = pending
	pendingMu.Unlock()

	// Notify both users about pending match
	sendToUser(u1, map[string]interface{}{
		"type":       "match_pending",
		"matchId":    matchID,
		"category":   finalCat,
		"difficulty": finalDiff,
		"expiresIn":  15,
	})

	sendToUser(u2, map[string]interface{}{
		"type":       "match_pending",
		"matchId":    matchID,
		"category":   finalCat,
		"difficulty": finalDiff,
		"expiresIn":  15,
	})
}

// --- Remove User ---
func removeUser(userId, category, difficulty string) {
	log.Printf("Removing user %s from queues", userId)
	rdb.Del(ctx, fmt.Sprintf("user:%s", userId))
	rdb.ZRem(ctx, fmt.Sprintf("queue:%s:%s", category, difficulty), userId)
	rdb.ZRem(ctx, fmt.Sprintf("queue:%s", category), userId)
	rdb.ZRem(ctx, "queue:all", userId)
}

func main() {
	rand.Seed(time.Now().UnixNano())
	
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "your-secret-key"
	}
	jwtSecret = []byte(secret)

	rdb = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	// Start background processes
	go subscribeToRedis()
	go matchmakingLoop()
	go pendingMatchExpirationLoop()

	http.HandleFunc("/join", joinHandler)
	http.HandleFunc("/cancel", cancelHandler)
	http.HandleFunc("/check", checkHandler)
	http.HandleFunc("/done", doneHandler)
	http.HandleFunc("/handshake", handshakeHandler)
	http.HandleFunc("/ws", wsHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("Listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}