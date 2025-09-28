package models

import (
	"gorm.io/gorm"
)

// User represents a registered user in the system.
type User struct {
	gorm.Model
	Username     string `gorm:"unique;not null" json:"username"`
	Email        string `gorm:"unique;not null" json:"email"`
	PasswordHash string `gorm:"not null" json:"-"`
}
