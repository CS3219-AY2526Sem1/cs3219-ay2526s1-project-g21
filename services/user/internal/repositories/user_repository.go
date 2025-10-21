package repositories

import (
	"errors"
	"peerprep/user/internal/models"
	"strconv"

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
	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return nil, err
	}
	err = r.DB.Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

func (r *UserRepository) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.DB.Where("LOWER(username) = LOWER(?)", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

func (r *UserRepository) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.DB.Where("LOWER(email) = LOWER(?)", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

func (r *UserRepository) UpdateUser(userID string, updates *models.User) (*models.User, error) {
	var user models.User
	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return nil, err
	}
	if err := r.DB.Where("id = ?", id).First(&user).Error; err != nil {
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
	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		return err
	}
	result := r.DB.Where("id = ?", id).Delete(&models.User{})
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return result.Error
}
