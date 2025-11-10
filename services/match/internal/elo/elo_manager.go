package elo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/redis/go-redis/v9"

	"match/internal/models"
)

const (
	// K-factors for Elo calculation
	KFactorNew         = 32.0 // Users with < 5 sessions
	KFactorExperienced = 24.0 // Users with >= 5 sessions

	// Default Elo rating
	DefaultElo = 1500.0

	// Redis key prefixes
	UserEloPrefix = "user_elo:"
)

// EloManager handles Elo rating calculations and updates
type EloManager struct {
	ctx context.Context
	rdb *redis.Client
}

// NewEloManager creates a new Elo manager
func NewEloManager(rdb *redis.Client) *EloManager {
	return &EloManager{
		ctx: context.Background(),
		rdb: rdb,
	}
}

// CalculateExpectedScore calculates the expected performance based on Elo difference
// Formula: 1 / (1 + 10^((opponentElo - userElo) / 400))
func CalculateExpectedScore(userElo, opponentElo float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, (opponentElo-userElo)/400.0))
}

// GetKFactor returns the appropriate K-factor based on session count
func GetKFactor(sessionsCompleted int) float64 {
	if sessionsCompleted < 5 {
		return KFactorNew
	}
	return KFactorExperienced
}

// CalculateNewElo calculates the new Elo rating based on engagement metrics
func CalculateNewElo(currentElo, opponentElo float64, engagementScore float64, sessionsCompleted int) float64 {
	expected := CalculateExpectedScore(currentElo, opponentElo)

	// Normalize engagement score to 0-1 range (since max is 100)
	actualPerformance := engagementScore / 100.0

	// Clamp to 0-1 to prevent extreme changes
	if actualPerformance > 1.0 {
		actualPerformance = 1.0
	}
	if actualPerformance < 0.0 {
		actualPerformance = 0.0
	}

	kFactor := GetKFactor(sessionsCompleted)

	// Calculate new Elo
	newElo := currentElo + kFactor*(actualPerformance-expected)

	// Ensure Elo doesn't go below 500 or above 3000
	if newElo < 500 {
		newElo = 500
	}
	if newElo > 3000 {
		newElo = 3000
	}

	return newElo
}

// UserEloInfo contains a user's Elo rating and session count
type UserEloInfo struct {
	UserID            string  `json:"userId"`
	EloRating         float64 `json:"eloRating"`
	SessionsCompleted int     `json:"sessionsCompleted"`
}

// GetUserElo retrieves a user's Elo rating from Redis (or returns default)
func (em *EloManager) GetUserElo(userId string) (*UserEloInfo, error) {
	key := fmt.Sprintf("%s%s", UserEloPrefix, userId)

	data, err := em.rdb.HGetAll(em.ctx, key).Result()
	if err == redis.Nil || len(data) == 0 {
		// User not found, return default
		return &UserEloInfo{
			UserID:            userId,
			EloRating:         DefaultElo,
			SessionsCompleted: 0,
		}, nil
	}
	if err != nil {
		return nil, err
	}

	info := &UserEloInfo{
		UserID: userId,
	}

	if eloStr, ok := data["elo_rating"]; ok {
		fmt.Sscanf(eloStr, "%f", &info.EloRating)
	} else {
		info.EloRating = DefaultElo
	}

	if sessionsStr, ok := data["sessions_completed"]; ok {
		fmt.Sscanf(sessionsStr, "%d", &info.SessionsCompleted)
	}

	return info, nil
}

// SetUserElo stores a user's Elo rating in Redis
func (em *EloManager) SetUserElo(userId string, elo float64, sessionsCompleted int) error {
	key := fmt.Sprintf("%s%s", UserEloPrefix, userId)

	err := em.rdb.HSet(em.ctx, key, map[string]interface{}{
		"elo_rating":         elo,
		"sessions_completed": sessionsCompleted,
		"last_updated":       time.Now().Unix(),
	}).Err()

	if err != nil {
		return fmt.Errorf("failed to set user Elo: %w", err)
	}

	// Set expiration to 90 days
	em.rdb.Expire(em.ctx, key, 90*24*time.Hour)

	return nil
}

// ProcessSessionMetrics calculates and updates Elo ratings for both users
func (em *EloManager) ProcessSessionMetrics(metrics *models.SessionMetrics) ([]*models.EloUpdate, error) {
	// Get current Elo for both users
	user1Info, err := em.GetUserElo(metrics.User1ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user1 Elo: %w", err)
	}

	user2Info, err := em.GetUserElo(metrics.User2ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user2 Elo: %w", err)
	}

	// Calculate engagement scores with difficulty adjustment
	user1Engagement := metrics.User1Metrics.CalculateEngagementScoreWithDifficulty(metrics.SessionDuration, metrics.Difficulty)
	user2Engagement := metrics.User2Metrics.CalculateEngagementScoreWithDifficulty(metrics.SessionDuration, metrics.Difficulty)

	log.Printf("[Elo] User1 %s engagement: %.2f (Elo: %.0f, Difficulty: %s)",
		metrics.User1ID, user1Engagement, user1Info.EloRating, metrics.Difficulty)
	log.Printf("[Elo] User2 %s engagement: %.2f (Elo: %.0f, Difficulty: %s)",
		metrics.User2ID, user2Engagement, user2Info.EloRating, metrics.Difficulty)

	// Calculate new Elo ratings
	newElo1 := CalculateNewElo(
		user1Info.EloRating,
		user2Info.EloRating,
		user1Engagement,
		user1Info.SessionsCompleted,
	)

	newElo2 := CalculateNewElo(
		user2Info.EloRating,
		user1Info.EloRating,
		user2Engagement,
		user2Info.SessionsCompleted,
	)

	// Update Redis
	err = em.SetUserElo(metrics.User1ID, newElo1, user1Info.SessionsCompleted+1)
	if err != nil {
		return nil, fmt.Errorf("failed to update user1 Elo: %w", err)
	}

	err = em.SetUserElo(metrics.User2ID, newElo2, user2Info.SessionsCompleted+1)
	if err != nil {
		return nil, fmt.Errorf("failed to update user2 Elo: %w", err)
	}

	// Create Elo update records
	updates := []*models.EloUpdate{
		{
			UserID:      metrics.User1ID,
			OldRating:   user1Info.EloRating,
			NewRating:   newElo1,
			Change:      newElo1 - user1Info.EloRating,
			OpponentElo: user2Info.EloRating,
			Engagement:  user1Engagement,
			Timestamp:   time.Now(),
		},
		{
			UserID:      metrics.User2ID,
			OldRating:   user2Info.EloRating,
			NewRating:   newElo2,
			Change:      newElo2 - user2Info.EloRating,
			OpponentElo: user1Info.EloRating,
			Engagement:  user2Engagement,
			Timestamp:   time.Now(),
		},
	}

	log.Printf("[Elo] User1 %s: %.0f -> %.0f (Δ%.0f)", metrics.User1ID, user1Info.EloRating, newElo1, newElo1-user1Info.EloRating)
	log.Printf("[Elo] User2 %s: %.0f -> %.0f (Δ%.0f)", metrics.User2ID, user2Info.EloRating, newElo2, newElo2-user2Info.EloRating)

	// Publish Elo update events
	for _, update := range updates {
		em.publishEloUpdate(update)
	}

	return updates, nil
}

// publishEloUpdate publishes an Elo update event to Redis
func (em *EloManager) publishEloUpdate(update *models.EloUpdate) {
	payload, err := json.Marshal(update)
	if err != nil {
		log.Printf("[Elo] Failed to marshal Elo update: %v", err)
		return
	}

	err = em.rdb.Publish(em.ctx, "elo_updates", payload).Err()
	if err != nil {
		log.Printf("[Elo] Failed to publish Elo update: %v", err)
	}
}

// CheckEloCompatibility checks if two users are within the Elo range for a given stage
func CheckEloCompatibility(elo1, elo2 float64, stage int) bool {
	diff := math.Abs(elo1 - elo2)

	switch stage {
	case 1:
		return diff <= 100
	case 2:
		return diff <= 200
	case 3:
		return diff <= 300
	default:
		return true // Stage 4 or higher, no Elo restriction
	}
}
