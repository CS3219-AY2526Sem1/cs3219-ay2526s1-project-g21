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

	"github.com/redis/go-redis/v9"
)

type RoomManager struct {
	rdb           *redis.Client
	questionURL   string
	roomStatusMap map[string]*models.RoomInfo
	mu            sync.RWMutex
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

	return &RoomManager{
		rdb:           rdb,
		questionURL:   questionURL,
		roomStatusMap: make(map[string]*models.RoomInfo),
	}
}

// SubscribeToMatches listens for match events until the provided context is cancelled.
func (ms *RoomManager) SubscribeToMatches(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	subscriber := ms.rdb.Subscribe(ctx, "matches")
	defer subscriber.Close()
	ch := subscriber.Channel()

	log.Println("Match service: Subscribed to match events")

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			ms.handleMatchPayload(msg.Payload)
		}
	}
}

func (ms *RoomManager) handleMatchPayload(payload string) {
	var event models.RoomInfo
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return
	}
	go ms.processMatchEvent(event)
}

// Process a match event by fetching question and creating room
func (ms *RoomManager) processMatchEvent(event models.RoomInfo) {
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

	ms.mu.Lock()
	ms.roomStatusMap[event.MatchId] = roomInfo
	ms.mu.Unlock()

	ms.updateRoomStatusInRedis(ctx, roomInfo)

	question, err := ms.fetchQuestion(event.Category, event.Difficulty)
	if err != nil {
		log.Printf("Failed to fetch question: %v", err)
		ms.mu.Lock()
		roomInfo.Status = "error"
		ms.mu.Unlock()
		ms.updateRoomStatusInRedis(ctx, roomInfo)
		return
	}

	ms.mu.Lock()
	roomInfo.Question = question
	roomInfo.Status = "ready"
	ms.mu.Unlock()

	ms.updateRoomStatusInRedis(ctx, roomInfo)

	log.Printf("Match service: Room %s is ready with question %d", event.MatchId, question.ID)
}

// Fetch a random question from the question service
func (ms *RoomManager) fetchQuestion(category string, difficulty string) (*models.Question, error) {
	url := fmt.Sprintf("%s/questions/random?difficulty=%s&topic=%s", ms.questionURL, difficulty, category)

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

func (ms *RoomManager) fetchAlternativeQuestion(category string, difficulty string, currentID int) (*models.Question, error) {
	for i := 0; i < maxFetchAttempts; i++ {
		question, err := ms.fetchQuestion(category, difficulty)
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
func (ms *RoomManager) updateRoomStatusInRedis(ctx context.Context, roomInfo *models.RoomInfo) {
	roomKey := "room:" + roomInfo.MatchId

	questionJSON := ""
	if roomInfo.Question != nil {
		if data, err := json.Marshal(roomInfo.Question); err == nil {
			questionJSON = string(data)
		}
	}

	ms.rdb.HSet(ctx, roomKey, map[string]interface{}{
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

	ms.rdb.Expire(ctx, roomKey, 24*time.Hour)
}

// Get room status by match ID
func (ms *RoomManager) GetRoomStatus(matchId string) (*models.RoomInfo, error) {
	ms.mu.RLock()
	if roomInfo, exists := ms.roomStatusMap[matchId]; exists {
		copy := cloneRoomInfo(roomInfo)
		ms.mu.RUnlock()
		return copy, nil
	}
	ms.mu.RUnlock()

	roomInfo, err := ms.fetchRoomStatusFromRedis(matchId)
	if err != nil {
		return nil, err
	}

	ms.mu.Lock()
	ms.roomStatusMap[matchId] = roomInfo
	ms.mu.Unlock()

	return cloneRoomInfo(roomInfo), nil
}

func (ms *RoomManager) fetchRoomStatusFromRedis(matchId string) (*models.RoomInfo, error) {
	ctx := context.Background()
	roomKey := "room:" + matchId

	result := ms.rdb.HGetAll(ctx, roomKey)
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
			log.Printf("Match service: Failed to decode question for room %s: %v", matchId, err)
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

func (ms *RoomManager) RerollQuestion(matchId string) (*models.RoomInfo, error) {
	ms.mu.Lock()
	roomInfo, exists := ms.roomStatusMap[matchId]
	ms.mu.Unlock()

	if !exists {
		loaded, err := ms.fetchRoomStatusFromRedis(matchId)
		if err != nil {
			return nil, err
		}

		ms.mu.Lock()
		roomInfo, exists = ms.roomStatusMap[matchId]
		if !exists {
			ms.roomStatusMap[matchId] = loaded
			roomInfo = loaded
		}
		ms.mu.Unlock()
	}

	ms.mu.Lock()
	if roomInfo.RerollsRemaining <= 0 {
		ms.mu.Unlock()
		return nil, ErrNoRerolls
	}
	roomInfo.RerollsRemaining--
	category := roomInfo.Category
	difficulty := roomInfo.Difficulty
	currentQuestionID := 0
	if roomInfo.Question != nil {
		currentQuestionID = roomInfo.Question.ID
	}
	ms.mu.Unlock()

	question, err := ms.fetchAlternativeQuestion(category, difficulty, currentQuestionID)
	if err != nil {
		ms.mu.Lock()
		roomInfo.RerollsRemaining++
		ms.mu.Unlock()
		log.Printf("Failed to reroll question for room %s: %v", matchId, err)
		return nil, err
	}

	ms.mu.Lock()
	roomInfo.Question = question
	roomInfo.Status = "ready"
	updatedCopy := cloneRoomInfo(roomInfo)
	ms.mu.Unlock()

	go ms.updateRoomStatusInRedis(context.Background(), roomInfo)

	log.Printf("Match service: Room %s rerolled question to %d", matchId, question.ID)

	return updatedCopy, nil
}

// ValidateRoomAccess validates if a user can access a room using their token
func (ms *RoomManager) ValidateRoomAccess(token string) (*models.RoomInfo, error) {
	claims, err := utils.ValidateRoomToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	roomInfo, err := ms.GetRoomStatus(claims.MatchId)
	if err != nil {
		return nil, fmt.Errorf("room not found: %w", err)
	}

	if claims.UserId != roomInfo.User1 && claims.UserId != roomInfo.User2 {
		return nil, fmt.Errorf("user not authorized for this room")
	}

	return roomInfo, nil
}

// PublishSessionEnded publishes a session ended event to Redis
func (ms *RoomManager) PublishSessionEnded(event models.SessionEndedEvent) error {
	ctx := context.Background()

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal session ended event: %w", err)
	}

	if err := ms.rdb.Publish(ctx, "session_ended", string(eventJSON)).Err(); err != nil {
		return fmt.Errorf("failed to publish session ended event: %w", err)
	}

	log.Printf("Published session_ended event for match %s", event.MatchID)
	return nil
}

// GetRoomInfoForSession retrieves room information for a session
func (ms *RoomManager) GetRoomInfoForSession(matchID string) (*models.RoomInfo, error) {
	return ms.GetRoomStatus(matchID)
}

// GetActiveRoomForUser checks if a user has an active room
func (ms *RoomManager) GetActiveRoomForUser(userID string) (*models.RoomInfo, error) {
	ctx := context.Background()

	// Search for rooms in Redis where user is either user1 or user2
	// We'll scan for room:* keys and check if the user is in any active room
	iter := ms.rdb.Scan(ctx, 0, "room:*", 0).Iterator()
	for iter.Next(ctx) {
		roomKey := iter.Val()
		roomMap := ms.rdb.HGetAll(ctx, roomKey).Val()

		// Check if user is in this room and room is active (ready status)
		if (roomMap["user1"] == userID || roomMap["user2"] == userID) && roomMap["status"] == "ready" {
			matchID := roomMap["matchId"]
			return ms.GetRoomStatus(matchID)
		}
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("no active room found for user")
}
