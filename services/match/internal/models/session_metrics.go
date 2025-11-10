package models

import "time"

// SessionMetrics captures engagement metrics from a completed session
type SessionMetrics struct {
	SessionID     string    `json:"sessionId"`
	MatchID       string    `json:"matchId"`
	User1ID       string    `json:"user1Id"`
	User2ID       string    `json:"user2Id"`

	// Question difficulty for Elo calculation
	Difficulty string `json:"difficulty"` // "easy", "medium", "hard"

	// Engagement metrics
	SessionDuration int `json:"sessionDuration"` // seconds

	// Per-user metrics
	User1Metrics UserSessionMetrics `json:"user1Metrics"`
	User2Metrics UserSessionMetrics `json:"user2Metrics"`

	Timestamp time.Time `json:"timestamp"`
}

// UserSessionMetrics tracks individual user engagement in a session
type UserSessionMetrics struct {
	VoiceUsed        bool `json:"voiceUsed"`
	VoiceDuration    int  `json:"voiceDuration"`    // seconds
	CodeChanges      int  `json:"codeChanges"`      // number of edits
	MessagesExchanged int  `json:"messagesExchanged"` // chat messages sent
}

// CalculateEngagementScore converts session metrics to a 0-100 engagement score
func (m *UserSessionMetrics) CalculateEngagementScore(sessionDuration int) float64 {
	score := 0.0

	// Voice usage (0-30 points)
	if m.VoiceUsed {
		score += 10.0
		// Additional points based on voice duration
		voiceRatio := float64(m.VoiceDuration) / float64(sessionDuration)
		if voiceRatio > 0.7 {
			score += 20.0
		} else if voiceRatio > 0.4 {
			score += 15.0
		} else if voiceRatio > 0.2 {
			score += 10.0
		} else {
			score += 5.0
		}
	}

	// Code changes (0-30 points)
	if m.CodeChanges >= 50 {
		score += 30.0
	} else if m.CodeChanges >= 20 {
		score += 25.0
	} else if m.CodeChanges >= 10 {
		score += 20.0
	} else if m.CodeChanges >= 5 {
		score += 15.0
	} else if m.CodeChanges > 0 {
		score += 10.0
	}

	// Session duration (0-30 points)
	// Optimal range: 10-45 minutes
	if sessionDuration >= 600 && sessionDuration <= 2700 {
		score += 30.0
	} else if sessionDuration >= 300 && sessionDuration <= 3600 {
		score += 25.0
	} else if sessionDuration >= 180 {
		score += 20.0
	} else if sessionDuration >= 120 {
		score += 10.0
	}

	// Messages exchanged (0-10 points)
	if m.MessagesExchanged >= 20 {
		score += 10.0
	} else if m.MessagesExchanged >= 10 {
		score += 7.0
	} else if m.MessagesExchanged >= 5 {
		score += 5.0
	} else if m.MessagesExchanged > 0 {
		score += 3.0
	}

	return score
}

// GetDifficultyMultiplier returns a multiplier based on question difficulty
// Harder questions should result in higher Elo gains
func GetDifficultyMultiplier(difficulty string) float64 {
	switch difficulty {
	case "easy":
		return 0.8 // 80% of base score
	case "medium":
		return 1.0 // 100% of base score (no change)
	case "hard":
		return 1.3 // 130% of base score (30% bonus)
	default:
		return 1.0 // Default to medium if unknown
	}
}

// CalculateEngagementScoreWithDifficulty calculates engagement score with difficulty adjustment
func (m *UserSessionMetrics) CalculateEngagementScoreWithDifficulty(sessionDuration int, difficulty string) float64 {
	baseScore := m.CalculateEngagementScore(sessionDuration)
	multiplier := GetDifficultyMultiplier(difficulty)

	// Apply multiplier and ensure we don't exceed 100
	adjustedScore := baseScore * multiplier
	if adjustedScore > 100.0 {
		adjustedScore = 100.0
	}

	return adjustedScore
}

// EloUpdate represents a user's Elo rating change
type EloUpdate struct {
	UserID       string    `json:"userId"`
	OldRating    float64   `json:"oldRating"`
	NewRating    float64   `json:"newRating"`
	Change       float64   `json:"change"`
	OpponentElo  float64   `json:"opponentElo"`
	Engagement   float64   `json:"engagement"`
	Timestamp    time.Time `json:"timestamp"`
}
