package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var (
	ctx         = context.Background()
	rdb         *redis.Client
	upgrader    = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	connections = make(map[*websocket.Conn]bool)
	mu          sync.Mutex
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

type MatchEvent struct {
	MatchId    string `json:"matchId"`
	User1      string `json:"user1"`
	User2      string `json:"user2"`
	Category   string `json:"category"`
	Difficulty string `json:"difficulty"`
}

// --- Helper ---
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

// --- WebSocket Handler ---
func wsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	mu.Lock()
	connections[conn] = true
	mu.Unlock()
	log.Println("New WebSocket client connected")

	for {
		if _, _, err := conn.NextReader(); err != nil {
			mu.Lock()
			delete(connections, conn)
			mu.Unlock()
			conn.Close()
			log.Println("Client disconnected")
			break
		}
	}
}

// --- Redis Subscriber ---
func subscribeToRedis() {
	subscriber := rdb.Subscribe(ctx, "matches")
	ch := subscriber.Channel()

	for msg := range ch {
		var event MatchEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Println("Failed to parse event:", err)
			continue
		}

		mu.Lock()
		for conn := range connections {
			if err := conn.WriteJSON(event); err != nil {
				conn.Close()
				delete(connections, conn)
			}
		}
		mu.Unlock()
	}
}

// --- Join Handler / Matchmaking ---
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

	// Add user to queues
	now := float64(time.Now().Unix())
	pipe := rdb.TxPipeline()
	pipe.ZAdd(ctx, fmt.Sprintf("queue:%s:%s", req.Category, req.Difficulty), redis.Z{Score: now, Member: req.UserID})
	pipe.ZAdd(ctx, fmt.Sprintf("queue:%s", req.Category), redis.Z{Score: now, Member: req.UserID})
	pipe.ZAdd(ctx, "queue:all", redis.Z{Score: now, Member: req.UserID})
	_, err := pipe.Exec(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, Resp{OK: false, Info: "redis error"})
		return
	}

	// Simple matchmaking: just pop 2 users from same category/difficulty
	queueKey := fmt.Sprintf("queue:%s:%s", req.Category, req.Difficulty)
	users, _ := rdb.ZRange(ctx, queueKey, 0, 1).Result()
	if len(users) < 2 {
		writeJSON(w, http.StatusOK, Resp{OK: true, Info: "enqueued"})
		return
	}

	u1, u2 := users[0], users[1]

	// Remove matched users from queue
	rdb.ZRem(ctx, queueKey, u1, u2)
	rdb.ZRem(ctx, fmt.Sprintf("queue:%s", req.Category), u1, u2)
	rdb.ZRem(ctx, "queue:all", u1, u2)

	matchID := uuid.New().String()
	match := MatchEvent{
		MatchId:    matchID,
		User1:      u1,
		User2:      u2,
		Category:   req.Category,
		Difficulty: req.Difficulty,
	}

	// Store match info in Redis
	matchKey := "match:" + matchID
	rdb.HSet(ctx, matchKey, map[string]interface{}{
		"id":         matchID,
		"user1":      u1,
		"user2":      u2,
		"category":   req.Category,
		"difficulty": req.Difficulty,
		"created_at": time.Now().Format(time.RFC3339),
		"status":     "pending", // Room creation pending
	})
	rdb.LPush(ctx, "matches", matchID)

	// Publish match event to Redis for collab service to process
	data, _ := json.Marshal(match)
	rdb.Publish(ctx, "matches", data)

	writeJSON(w, http.StatusOK, Resp{OK: true, Info: match})
}

func main() {
	rdb = redis.NewClient(&redis.Options{
		Addr: "redis:6379",
	})

	// Subscribe to Redis in background
	go subscribeToRedis()

	http.HandleFunc("/join", joinHandler)
	http.HandleFunc("/ws", wsHandler)

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
