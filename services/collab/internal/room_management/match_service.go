package room_management

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"collab/internal/models"
	"collab/internal/utils"

	"github.com/redis/go-redis/v9"
)

type RoomManager struct {
	rdb           *redis.Client
	questionURL   string
	roomStatusMap map[string]*models.RoomInfo
}

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

// Subscribe to match events from Redis
func (ms *RoomManager) SubscribeToMatches() {
	ctx := context.Background()
	subscriber := ms.rdb.Subscribe(ctx, "matches")
	ch := subscriber.Channel()

	log.Println("Match service: Subscribed to match events")

	for msg := range ch {
		var event models.RoomInfo
		if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
			log.Printf("Match service: Failed to parse event: %v", err)
			continue
		}

		log.Printf("Match service: Received match event: %+v", event)
		go ms.processMatchEvent(event)
	}
}

// Process a match event by fetching question and creating room
func (ms *RoomManager) processMatchEvent(event models.RoomInfo) {
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
	ms.updateRoomStatusInRedis(ctx, roomInfo, event.Token1, event.Token2)

	// Fetch question from question service
	question, err := ms.fetchQuestion(event.Category, event.Difficulty)
	if err != nil {
		log.Printf("Failed to fetch question: %v", err)
		roomInfo.Status = "error"
		ms.updateRoomStatusInRedis(ctx, roomInfo, event.Token1, event.Token2)
		return
	}

	// Update room with question info
	roomInfo.Question = question
	roomInfo.Status = "ready"
	ms.roomStatusMap[event.MatchId] = roomInfo

	// Update Redis with ready status
	ms.updateRoomStatusInRedis(ctx, roomInfo, event.Token1, event.Token2)

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

// Update room status in Redis
func (ms *RoomManager) updateRoomStatusInRedis(ctx context.Context, roomInfo *models.RoomInfo, token1, token2 string) {
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
		"token1":     token1,
		"token2":     token2,
		"question":   string(data),
		"createdAt":  roomInfo.CreatedAt,
	})

	// Set expiration for room data (24 hours)
	ms.rdb.Expire(ctx, roomKey, 24*time.Hour)
}

// Get room status by match ID
func (ms *RoomManager) GetRoomStatus(matchId string) (*models.RoomInfo, error) {
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

// ValidateRoomAccess validates if a user can access a room using their token
func (ms *RoomManager) ValidateRoomAccess(token string) (*models.RoomInfo, error) {
	// Validate the JWT token
	claims, err := utils.ValidateRoomToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Get room info
	roomInfo, err := ms.GetRoomStatus(claims.MatchId)
	if err != nil {
		return nil, fmt.Errorf("room not found: %w", err)
	}

	// Verify the user is one of the matched users
	if claims.UserId != roomInfo.User1 && claims.UserId != roomInfo.User2 {
		return nil, fmt.Errorf("user not authorized for this room")
	}

	return roomInfo, nil
}
