package models

import (
	"time"

	"gorm.io/gorm"
)

// AIFeedback stores user feedback on AI-generated responses for fine-tuning
// Note: User IDs are intentionally excluded for privacy
type AIFeedback struct {
	gorm.Model
	RequestID    string     `gorm:"uniqueIndex;not null" json:"request_id"`
	RequestType  string     `gorm:"not null" json:"request_type"` // "hint", "explanation", "solution"
	Prompt       string     `gorm:"type:text;not null" json:"prompt"`
	Response     string     `gorm:"type:text;not null" json:"response"`
	IsPositive   bool       `gorm:"not null" json:"is_positive"` // true = thumbs up, false = thumbs down
	ModelVersion string     `gorm:"not null" json:"model_version"`
	FeedbackAt   time.Time  `gorm:"not null" json:"feedback_at"`
	Exported     bool       `gorm:"not null;default:false;index" json:"exported"`
	ExportedAt   *time.Time `json:"exported_at"`
}

// TrainingDataPoint represents a single training example in JSONL format for Gemini fine-tuning
type TrainingDataPoint struct {
	Contents []TrainingContent `json:"contents"`
}

type TrainingContent struct {
	Role  string         `json:"role"` // "user" or "model"
	Parts []TrainingPart `json:"parts"`
}

type TrainingPart struct {
	Text string `json:"text"`
}

// ModelVersion tracks deployed model versions for A/B testing
type ModelVersion struct {
	gorm.Model
	VersionName      string     `gorm:"uniqueIndex;not null" json:"version_name"` // e.g., "gemini-1.5-flash-ft-001"
	BaseModel        string     `gorm:"not null" json:"base_model"`
	TrainingJobID    string     `gorm:"uniqueIndex" json:"training_job_id"`
	TrainingDataSize int        `gorm:"not null" json:"training_data_size"`
	IsActive         bool       `gorm:"not null;default:false" json:"is_active"`
	TrafficWeight    int        `gorm:"not null;default:0" json:"traffic_weight"` // 0-100 percentage
	ActivatedAt      *time.Time `json:"activated_at"`
	DeactivatedAt    *time.Time `json:"deactivated_at"`
}

// RequestContext stores request/response pairs temporarily for feedback collection
// This is stored in-memory with TTL, not in database
type RequestContext struct {
	RequestID    string
	RequestType  string
	Prompt       string
	Response     string
	ModelVersion string
	Timestamp    time.Time
}
