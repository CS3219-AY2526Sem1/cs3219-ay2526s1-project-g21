package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"peerprep/user/internal/handlers"
	"peerprep/user/internal/models"

	"github.com/redis/go-redis/v9"
)

type SessionEndedEvent struct {
	MatchID       string `json:"matchId"`
	User1         string `json:"user1"`
	User2         string `json:"user2"`
	QuestionID    int    `json:"questionId"`
	QuestionTitle string `json:"questionTitle"`
	Category      string `json:"category"`
	Difficulty    string `json:"difficulty"`
	Language      string `json:"language"`
	FinalCode     string `json:"finalCode"`
	StartedAt     string `json:"startedAt"`
	EndedAt       string `json:"endedAt"`
	DurationSec   int    `json:"durationSeconds"`
	RerollsUsed   int    `json:"rerollsUsed"`
}

type HistorySubscriber struct {
	rdb            *redis.Client
	historyHandler *handlers.HistoryHandler
}

func NewHistorySubscriber(redisAddr string, historyHandler *handlers.HistoryHandler) *HistorySubscriber {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	return &HistorySubscriber{
		rdb:            rdb,
		historyHandler: historyHandler,
	}
}

// SubscribeToSessionEnded listens for session ended events from Redis
func (hs *HistorySubscriber) SubscribeToSessionEnded(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	subscriber := hs.rdb.Subscribe(ctx, "session_ended")
	defer subscriber.Close()
	ch := subscriber.Channel()

	log.Println("History subscriber: Subscribed to session_ended events")

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			hs.handleSessionEndedEvent(msg.Payload)
		}
	}
}

func (hs *HistorySubscriber) handleSessionEndedEvent(payload string) {
	var event SessionEndedEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		log.Printf("Failed to unmarshal session ended event: %v", err)
		return
	}

	log.Printf("Received session_ended event for match %s", event.MatchID)

	// Parse timestamps
	startedAt, err := time.Parse(time.RFC3339, event.StartedAt)
	if err != nil {
		log.Printf("Failed to parse startedAt: %v", err)
		startedAt = time.Now()
	}

	endedAt, err := time.Parse(time.RFC3339, event.EndedAt)
	if err != nil {
		log.Printf("Failed to parse endedAt: %v", err)
		endedAt = time.Now()
	}

	// Create interview history record
	history := &models.InterviewHistory{
		MatchID:       event.MatchID,
		User1ID:       event.User1,
		User2ID:       event.User2,
		QuestionID:    event.QuestionID,
		QuestionTitle: event.QuestionTitle,
		Category:      event.Category,
		Difficulty:    event.Difficulty,
		Language:      event.Language,
		FinalCode:     event.FinalCode,
		StartedAt:     startedAt,
		EndedAt:       endedAt,
		DurationSec:   event.DurationSec,
		RerollsUsed:   event.RerollsUsed,
	}

	// Save to database
	if err := hs.historyHandler.CreateHistory(history); err != nil {
		log.Printf("Failed to save interview history for match %s: %v", event.MatchID, err)
		return
	}

	log.Printf("Successfully saved interview history for match %s", event.MatchID)
}
