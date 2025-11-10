package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a registered user in the system.
type User struct {
	gorm.Model
	Username     string  `gorm:"unique;not null" json:"username"`
	Email        string  `gorm:"unique;not null" json:"email"`
	PasswordHash string  `gorm:"not null" json:"-"`
	Verified     bool    `gorm:"not null;default:false" json:"verified"`
	NewEmail     *string `gorm:"uniqueIndex:new_email_idx" json:"-"`

	// Elo rating fields (hidden from users, used for matchmaking)
	EloRating         float64    `gorm:"default:1500" json:"-"` // Hidden from JSON response
	SessionsCompleted int        `gorm:"default:0" json:"-"`
	LastEloUpdate     *time.Time `json:"-"`
}

// TokenPurpose indicates why a token exists
type TokenPurpose string

const (
	TokenPurposeAccountVerification TokenPurpose = "account_verification"
	TokenPurposeEmailChange         TokenPurpose = "email_change"
)

// Token stores short-lived verification tokens for specific purposes
type Token struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Token     string       `gorm:"uniqueIndex;not null"`
	Purpose   TokenPurpose `gorm:"type:varchar(32);not null"`
	UserID    uint         `gorm:"not null;index"`
	ExpiresAt time.Time    `gorm:"index;not null"`
}
