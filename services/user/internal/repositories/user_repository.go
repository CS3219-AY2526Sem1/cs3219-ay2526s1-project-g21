package repositories

import (
	"errors"
	"peerprep/user/internal/models"

	"gorm.io/gorm"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	DB *gorm.DB
}

func (r *UserRepository) CreateUser(user *models.User) error {
	return r.DB.Create(user).Error
}

func (r *UserRepository) GetUserByID(userID string) (*models.User, error) {
	var user models.User
	err := r.DB.First(&user, "user_id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

func (r *UserRepository) UpdateUser(userID string, updates *models.User) (*models.User, error) {
	var user models.User
	if err := r.DB.First(&user, "user_id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if err := r.DB.Model(&user).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) DeleteUser(userID string) error {
	result := r.DB.Delete(&models.User{}, "user_id = ?", userID)
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return result.Error
}
