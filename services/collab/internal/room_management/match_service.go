package room_management

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"collab/internal/models"
	"collab/internal/utils"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RoomManager struct {
	rdb           *redis.Client
	pubClient     *redis.Client
	subClient     *redis.Client
	questionURL   string
	roomStatusMap map[string]*models.RoomInfo
	mu            sync.RWMutex
	instanceID    string
	ctx           context.Context

	// Callback for room update events (set by handlers)
	onRoomUpdate func(matchId string, roomInfo *models.RoomInfo)
}

const (
	maxFetchAttempts = 5
)

var (
	ErrNoRerolls             = errors.New("no rerolls remaining")
	ErrNoAlternativeQuestion = errors.New("no alternative question available")
)

func NewRoomManager(redisAddr, questionURL string) *RoomManager {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Separate client for pub/sub
	subClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	rm := &RoomManager{
		rdb:           rdb,
		pubClient:     rdb,
		subClient:     subClient,
		questionURL:   questionURL,
		roomStatusMap: make(map[string]*models.RoomInfo),
		instanceID:    uuid.New().String()[:8], // Short instance ID for logging
		ctx:           context.Background(),
	}

	log.Printf("[RoomManager %s] Initialized", rm.instanceID)

	return rm
}

func (rm *RoomManager) GetInstanceID() string {
	return rm.instanceID
}

// SetRoomUpdateCallback sets the callback for when room updates are received
// This is typically called by the handlers to broadcast updates to WebSocket clients
func (rm *RoomManager) SetRoomUpdateCallback(callback func(matchId string, roomInfo *models.RoomInfo)) {
	rm.onRoomUpdate = callback
}

// GetRedisClient returns the Redis client (needed for handlers to subscribe)
func (rm *RoomManager) GetRedisClient() *redis.Client {
	return rm.rdb
}

// SubscribeToMatches listens for match events until the provided context is cancelled
func (rm *RoomManager) SubscribeToMatches(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	subscriber := rm.rdb.Subscribe(ctx, "matches")
	defer subscriber.Close()
	ch := subscriber.Channel()

	log.Printf("[RoomManager %s] Subscribed to match events", rm.instanceID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[RoomManager %s] Stopping match subscription", rm.instanceID)
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			rm.handleMatchPayload(msg.Payload)
		}
	}
}

// SubscribeToRoomUpdates listens for room update events (rerolls, status changes)
// This allows instances without active WebSocket connections to stay in sync
func (rm *RoomManager) SubscribeToRoomUpdates(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	subscriber := rm.subClient.Subscribe(ctx, "room_updates")
	defer subscriber.Close()
	ch := subscriber.Channel()

	log.Printf("[RoomManager %s] Subscribed to room update events", rm.instanceID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[RoomManager %s] Stopping room updates subscription", rm.instanceID)
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			rm.handleRoomUpdateEvent(msg.Payload)
		}
	}
}

func (rm *RoomManager) handleMatchPayload(payload string) {
	var event models.RoomInfo
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("[RoomManager %s] Failed to parse match event: %v", rm.instanceID, err)
		return
	}
	go rm.processMatchEvent(event)
}

func (rm *RoomManager) handleRoomUpdateEvent(payload string) {
	var event struct {
		Type       string           `json:"type"`
		InstanceID string           `json:"instanceId"`
		MatchId    string           `json:"matchId"`
		RoomInfo   *models.RoomInfo `json:"roomInfo"`
	}

	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("[RoomManager %s] Failed to parse room update event: %v", rm.instanceID, err)
		return
	}

	// Ignore our own events
	if event.InstanceID == rm.instanceID {
		return
	}

	log.Printf("[RoomManager %s] Received room update from instance %s for match %s",
		rm.instanceID, event.InstanceID, event.MatchId)

	// Update local cache
	rm.mu.Lock()
	rm.roomStatusMap[event.MatchId] = event.RoomInfo
	rm.mu.Unlock()

	// Trigger callback if set (handlers will use this to broadcast to WebSocket clients)
	if rm.onRoomUpdate != nil {
		rm.onRoomUpdate(event.MatchId, event.RoomInfo)
	}
}

// Process a match event by fetching question and creating room
func (rm *RoomManager) processMatchEvent(event models.RoomInfo) {
	ctx := context.Background()

	roomInfo := &models.RoomInfo{
		MatchId:          event.MatchId,
		User1:            event.User1,
		User2:            event.User2,
		Category:         event.Category,
		Difficulty:       event.Difficulty,
		Status:           "processing",
		RerollsRemaining: 1,
		CreatedAt:        time.Now().Format(time.RFC3339),
		Token1:           event.Token1,
		Token2:           event.Token2,
	}

	rm.mu.Lock()
	rm.roomStatusMap[event.MatchId] = roomInfo
	rm.mu.Unlock()

	rm.updateRoomStatusInRedis(ctx, roomInfo)

	log.Printf("[RoomManager %s] Processing match %s", rm.instanceID, event.MatchId)

	question, err := rm.fetchQuestion(event.Category, event.Difficulty)
	if err != nil {
		log.Printf("[RoomManager %s] Failed to fetch question for match %s: %v",
			rm.instanceID, event.MatchId, err)
		rm.mu.Lock()
		roomInfo.Status = "error"
		rm.mu.Unlock()
		rm.updateRoomStatusInRedis(ctx, roomInfo)
		return
	}

	rm.mu.Lock()
	roomInfo.Question = question
	roomInfo.Status = "ready"
	rm.mu.Unlock()

	rm.updateRoomStatusInRedis(ctx, roomInfo)

	// Publish room update event
	rm.publishRoomUpdate(event.MatchId, roomInfo)

	log.Printf("[RoomManager %s] Room %s is ready with question %d",
		rm.instanceID, event.MatchId, question.ID)
}

// Fetch a random question from the question service
func (rm *RoomManager) fetchQuestion(category string, difficulty string) (*models.Question, error) {
	url := fmt.Sprintf("%s/api/v1/questions/random?difficulty=%s&topic=%s",
		rm.questionURL, difficulty, category)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to call question service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("question service returned status %d", resp.StatusCode)
	}

	var question models.Question
	if err := json.NewDecoder(resp.Body).Decode(&question); err != nil {
		return nil, fmt.Errorf("failed to decode question response: %w", err)
	}

	return &question, nil
}

func (rm *RoomManager) fetchAlternativeQuestion(category string, difficulty string, currentID int) (*models.Question, error) {
	for i := 0; i < maxFetchAttempts; i++ {
		question, err := rm.fetchQuestion(category, difficulty)
		if err != nil {
			return nil, err
		}
		if currentID == 0 || question.ID != currentID {
			return question, nil
		}
	}
	return nil, ErrNoAlternativeQuestion
}

// Update room status in Redis
func (rm *RoomManager) updateRoomStatusInRedis(ctx context.Context, roomInfo *models.RoomInfo) {
	roomKey := "room:" + roomInfo.MatchId

	questionJSON := ""
	if roomInfo.Question != nil {
		if data, err := json.Marshal(roomInfo.Question); err == nil {
			questionJSON = string(data)
		}
	}

	rm.rdb.HSet(ctx, roomKey, map[string]interface{}{
		"matchId":          roomInfo.MatchId,
		"user1":            roomInfo.User1,
		"user2":            roomInfo.User2,
		"category":         roomInfo.Category,
		"difficulty":       roomInfo.Difficulty,
		"status":           roomInfo.Status,
		"token1":           roomInfo.Token1,
		"token2":           roomInfo.Token2,
		"question":         questionJSON,
		"rerollsRemaining": roomInfo.RerollsRemaining,
		"createdAt":        roomInfo.CreatedAt,
	})

	rm.rdb.Expire(ctx, roomKey, 24*time.Hour)
}

// publishRoomUpdate publishes room update to Redis for other instances
func (rm *RoomManager) publishRoomUpdate(matchId string, roomInfo *models.RoomInfo) {
	event := struct {
		Type       string           `json:"type"`
		InstanceID string           `json:"instanceId"`
		MatchId    string           `json:"matchId"`
		RoomInfo   *models.RoomInfo `json:"roomInfo"`
	}{
		Type:       "room_updated",
		InstanceID: rm.instanceID,
		MatchId:    matchId,
		RoomInfo:   roomInfo,
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("[RoomManager %s] Failed to marshal room update event: %v", rm.instanceID, err)
		return
	}

	if err := rm.pubClient.Publish(rm.ctx, "room_updates", string(eventJSON)).Err(); err != nil {
		log.Printf("[RoomManager %s] Failed to publish room update: %v", rm.instanceID, err)
		return
	}

	log.Printf("[RoomManager %s] Published room update for match %s", rm.instanceID, matchId)
}

// Get room status by match ID
func (rm *RoomManager) GetRoomStatus(matchId string) (*models.RoomInfo, error) {
	rm.mu.RLock()
	if roomInfo, exists := rm.roomStatusMap[matchId]; exists {
		copy := cloneRoomInfo(roomInfo)
		rm.mu.RUnlock()
		return copy, nil
	}
	rm.mu.RUnlock()

	roomInfo, err := rm.fetchRoomStatusFromRedis(matchId)
	if err != nil {
		return nil, err
	}

	rm.mu.Lock()
	rm.roomStatusMap[matchId] = roomInfo
	rm.mu.Unlock()

	return cloneRoomInfo(roomInfo), nil
}

func (rm *RoomManager) fetchRoomStatusFromRedis(matchId string) (*models.RoomInfo, error) {
	ctx := context.Background()
	roomKey := "room:" + matchId

	result := rm.rdb.HGetAll(ctx, roomKey)
	if result.Err() != nil {
		return nil, fmt.Errorf("failed to get room from Redis: %w", result.Err())
	}

	roomMap := result.Val()
	if len(roomMap) == 0 {
		return nil, fmt.Errorf("room not found")
	}

	roomInfo := &models.RoomInfo{
		MatchId:          roomMap["matchId"],
		User1:            roomMap["user1"],
		User2:            roomMap["user2"],
		Category:         roomMap["category"],
		Difficulty:       roomMap["difficulty"],
		Status:           roomMap["status"],
		RerollsRemaining: 0,
		CreatedAt:        roomMap["createdAt"],
		Token1:           roomMap["token1"],
		Token2:           roomMap["token2"],
	}

	if val := roomMap["rerollsRemaining"]; val != "" {
		if rr, err := strconv.Atoi(val); err == nil {
			roomInfo.RerollsRemaining = rr
		}
	}

	if questionData := roomMap["question"]; questionData != "" {
		var question models.Question
		if err := json.Unmarshal([]byte(questionData), &question); err != nil {
			log.Printf("[RoomManager %s] Failed to decode question for room %s: %v",
				rm.instanceID, matchId, err)
		} else {
			roomInfo.Question = &question
		}
	}

	return roomInfo, nil
}

func cloneRoomInfo(src *models.RoomInfo) *models.RoomInfo {
	if src == nil {
		return nil
	}
	copy := *src
	if src.Question != nil {
		qCopy := *src.Question
		copy.Question = &qCopy
	}
	return &copy
}

func (rm *RoomManager) RerollQuestion(matchId string) (*models.RoomInfo, error) {
	log.Printf("[RoomManager %s] Reroll requested for match %s", rm.instanceID, matchId)

	rm.mu.Lock()
	roomInfo, exists := rm.roomStatusMap[matchId]
	rm.mu.Unlock()

	if !exists {
		loaded, err := rm.fetchRoomStatusFromRedis(matchId)
		if err != nil {
			return nil, err
		}

		rm.mu.Lock()
		roomInfo, exists = rm.roomStatusMap[matchId]
		if !exists {
			rm.roomStatusMap[matchId] = loaded
			roomInfo = loaded
		}
		rm.mu.Unlock()
	}

	rm.mu.Lock()
	if roomInfo.RerollsRemaining <= 0 {
		rm.mu.Unlock()
		return nil, ErrNoRerolls
	}
	roomInfo.RerollsRemaining--
	category := roomInfo.Category
	difficulty := roomInfo.Difficulty
	currentQuestionID := 0
	if roomInfo.Question != nil {
		currentQuestionID = roomInfo.Question.ID
	}
	rm.mu.Unlock()

	question, err := rm.fetchAlternativeQuestion(category, difficulty, currentQuestionID)
	if err != nil {
		rm.mu.Lock()
		roomInfo.RerollsRemaining++
		rm.mu.Unlock()
		log.Printf("[RoomManager %s] Failed to reroll question for room %s: %v",
			rm.instanceID, matchId, err)
		return nil, err
	}

	rm.mu.Lock()
	roomInfo.Question = question
	roomInfo.Status = "ready"
	updatedCopy := cloneRoomInfo(roomInfo)
	rm.mu.Unlock()

	if rm.onRoomUpdate != nil {
		rm.onRoomUpdate(matchId, cloneRoomInfo(updatedCopy))
	}

	// Update Redis
	go rm.updateRoomStatusInRedis(context.Background(), roomInfo)

	// Publish room update event so other instances can broadcast to their WebSocket clients
	rm.publishRoomUpdate(matchId, updatedCopy)

	log.Printf("[RoomManager %s] Room %s rerolled question to %d",
		rm.instanceID, matchId, question.ID)

	return updatedCopy, nil
}

// ValidateRoomAccess validates if a user can access a room using their token
func (rm *RoomManager) ValidateRoomAccess(token string) (*models.RoomInfo, error) {
	claims, err := utils.ValidateRoomToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	roomInfo, err := rm.GetRoomStatus(claims.MatchId)
	if err != nil {
		return nil, fmt.Errorf("room not found: %w", err)
	}

	if claims.UserId != roomInfo.User1 && claims.UserId != roomInfo.User2 {
		return nil, fmt.Errorf("user not authorized for this room")
	}

	return roomInfo, nil
}

// PublishSessionEnded publishes a session ended event to Redis
func (rm *RoomManager) PublishSessionEnded(event models.SessionEndedEvent) error {
	ctx := context.Background()

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal session ended event: %w", err)
	}

	if err := rm.rdb.Publish(ctx, "session_ended", string(eventJSON)).Err(); err != nil {
		return fmt.Errorf("failed to publish session ended event: %w", err)
	}

	log.Printf("[RoomManager %s] Published session_ended event for match %s",
		rm.instanceID, event.MatchID)
	return nil
}

// GetRoomInfoForSession retrieves room information for a session
func (rm *RoomManager) GetRoomInfoForSession(matchID string) (*models.RoomInfo, error) {
	return rm.GetRoomStatus(matchID)
}

// GetActiveRoomForUser checks if a user has an active room
func (rm *RoomManager) GetActiveRoomForUser(userID string) (*models.RoomInfo, error) {
	ctx := context.Background()

	// Search for rooms in Redis where user is either user1 or user2
	iter := rm.rdb.Scan(ctx, 0, "room:*", 0).Iterator()
	for iter.Next(ctx) {
		roomKey := iter.Val()
		roomMap := rm.rdb.HGetAll(ctx, roomKey).Val()

		// Check if user is in this room and room is active (ready status)
		if (roomMap["user1"] == userID || roomMap["user2"] == userID) && roomMap["status"] == "ready" {
			matchID := roomMap["matchId"]
			return rm.GetRoomStatus(matchID)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("no active room found for user")
}

// MarkRoomAsEnded updates the room status to "ended" in Redis
func (rm *RoomManager) MarkRoomAsEnded(matchID string) error {
	ctx := context.Background()
	roomKey := "room:" + matchID

	// Update the status field to "ended"
	if err := rm.rdb.HSet(ctx, roomKey, "status", "ended").Err(); err != nil {
		return fmt.Errorf("failed to mark room as ended: %w", err)
	}

	// Update in-memory cache
	rm.mu.Lock()
	if roomInfo, exists := rm.roomStatusMap[matchID]; exists {
		roomInfo.Status = "ended"
		// Publish update
		go rm.publishRoomUpdate(matchID, roomInfo)
	}
	rm.mu.Unlock()

	// Set a shorter TTL (1 hour) for ended rooms
	rm.rdb.Expire(ctx, roomKey, 1*time.Hour)

	log.Printf("[RoomManager %s] Marked room %s as ended", rm.instanceID, matchID)
	return nil
}

// Cleanup closes Redis connections
func (rm *RoomManager) Cleanup() {
	rm.subClient.Close()
	log.Printf("[RoomManager %s] Cleaned up Redis connections", rm.instanceID)
}
