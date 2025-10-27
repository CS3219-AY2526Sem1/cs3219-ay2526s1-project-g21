package handlers

import "peerprep/user/internal/models"

// UserRepository captures the persistence operations required by handlers.
type UserRepository interface {
	CreateUser(user *models.User) error
	GetUserByUsername(username string) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)
	GetUserByID(userID string) (*models.User, error)
	UpdateUser(userID string, updates *models.User) (*models.User, error)
	DeleteUser(userID string) error
}
