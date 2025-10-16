package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"collab/internal/models"

	"github.com/redis/go-redis/v9"
)

type MatchService struct {
	rdb           *redis.Client
	questionURL   string
	roomStatusMap map[string]*models.RoomInfo
}

func NewMatchService(redisAddr, questionURL string) *MatchService {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	return &MatchService{
		rdb:           rdb,
		questionURL:   questionURL,
		roomStatusMap: make(map[string]*models.RoomInfo),
	}
}

// Subscribe to match events from Redis
func (ms *MatchService) SubscribeToMatches() {
	ctx := context.Background()
	subscriber := ms.rdb.Subscribe(ctx, "matches")
	ch := subscriber.Channel()

	log.Println("Match service: Subscribed to match events")

	for msg := range ch {
		var event models.MatchEvent
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Printf("Match service: Failed to parse event: %v", err)
			continue
		}

		log.Printf("Match service: Received match event: %+v", event)
		go ms.processMatchEvent(event)
	}
}

// Process a match event by fetching question and creating room
func (ms *MatchService) processMatchEvent(event models.MatchEvent) {
	ctx := context.Background()

	// Update room status to processing
	roomInfo := &models.RoomInfo{
		MatchId:    event.MatchId,
		User1:      event.User1,
		User2:      event.User2,
		Category:   event.Category,
		Difficulty: event.Difficulty,
		Status:     "processing",
		CreatedAt:  time.Now().Format(time.RFC3339),
	}

	// Store in memory for quick access
	ms.roomStatusMap[event.MatchId] = roomInfo

	// Update Redis with processing status
	ms.updateRoomStatusInRedis(ctx, roomInfo)

	// Fetch question from question service
	question, err := ms.fetchQuestion(event.Category, event.Difficulty)
	if err != nil {
		log.Printf("Match service: Failed to fetch question: %v", err)
		roomInfo.Status = "error"
		ms.updateRoomStatusInRedis(ctx, roomInfo)
		return
	}

	// Update room with question info
	roomInfo.Question = question
	roomInfo.Status = "ready"
	ms.roomStatusMap[event.MatchId] = roomInfo

	// Update Redis with ready status
	ms.updateRoomStatusInRedis(ctx, roomInfo)

	log.Printf("Match service: Room %s is ready with question %d", event.MatchId, question.ID)
}

// Fetch a random question from the question service
func (ms *MatchService) fetchQuestion(category, difficulty string) (*models.Question, error) {
	url := fmt.Sprintf("%s/questions/random?difficulty=%s", ms.questionURL, difficulty)

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

// Update room status in Redis
func (ms *MatchService) updateRoomStatusInRedis(ctx context.Context, roomInfo *models.RoomInfo) {
	roomKey := "room:" + roomInfo.MatchId

	data, err := json.Marshal(roomInfo)
	if err != nil {
		log.Printf("Match service: Failed to marshal room info: %v", err)
		return
	}

	ms.rdb.HSet(ctx, roomKey, map[string]interface{}{
		"matchId":    roomInfo.MatchId,
		"user1":      roomInfo.User1,
		"user2":      roomInfo.User2,
		"category":   roomInfo.Category,
		"difficulty": roomInfo.Difficulty,
		"status":     roomInfo.Status,
		"question":   string(data),
		"createdAt":  roomInfo.CreatedAt,
	})

	// Set expiration for room data (24 hours)
	ms.rdb.Expire(ctx, roomKey, 24*time.Hour)
}

// Get room status by match ID
func (ms *MatchService) GetRoomStatus(matchId string) (*models.RoomInfo, error) {
	// First check memory cache
	if roomInfo, exists := ms.roomStatusMap[matchId]; exists {
		return roomInfo, nil
	}

	// Fallback to Redis
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
		MatchId:    roomMap["matchId"],
		User1:      roomMap["user1"],
		User2:      roomMap["user2"],
		Category:   roomMap["category"],
		Difficulty: roomMap["difficulty"],
		Status:     roomMap["status"],
		CreatedAt:  roomMap["createdAt"],
	}

	// Parse question if present
	if questionData := roomMap["question"]; questionData != "" {
		var question models.Question
		if err := json.Unmarshal([]byte(questionData), &question); err == nil {
			roomInfo.Question = &question
		}
	}

	return roomInfo, nil
}
