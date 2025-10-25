package models

import (
	"time"

	"gorm.io/gorm"
)

// InterviewHistory represents a completed interview session.
type InterviewHistory struct {
	gorm.Model
	MatchID        string    `gorm:"not null;index" json:"matchId"`
	User1ID        string    `gorm:"not null;index" json:"user1Id"`
	User2ID        string    `gorm:"not null;index" json:"user2Id"`
	QuestionID     int       `json:"questionId"`
	QuestionTitle  string    `json:"questionTitle"`
	Category       string    `json:"category"`
	Difficulty     string    `json:"difficulty"`
	Language       string    `json:"language"`
	FinalCode      string    `gorm:"type:text" json:"finalCode"`
	StartedAt      time.Time `json:"startedAt"`
	EndedAt        time.Time `json:"endedAt"`
	DurationSec    int       `json:"durationSeconds"`
	RerollsUsed    int       `gorm:"default:0" json:"rerollsUsed"`
}
