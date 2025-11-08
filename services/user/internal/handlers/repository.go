package handlers

import (
	"peerprep/user/internal/models"
	"time"
)

// UserRepository captures the persistence operations required by handlers.
type UserRepository interface {
	CreateUser(user *models.User) error
	GetUserByUsername(username string) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)
	GetUserByID(userID string) (*models.User, error)
	UpdateUser(userID string, updates *models.User) (*models.User, error)
	DeleteUser(userID string) error
}

// TokenRepository captures the token persistence operations required by handlers.
type TokenRepository interface {
	Create(token *models.Token) error
	GetByToken(tokenStr string) (*models.Token, error)
	GetByUserAndPurpose(userID uint, purpose models.TokenPurpose) (*models.Token, error)
	DeleteByID(id uint) error
	DeleteByToken(tokenStr string) error
	DeleteByUserAndPurpose(userID uint, purpose models.TokenPurpose) error
	DeleteExpired(before time.Time) (int64, error)
}
