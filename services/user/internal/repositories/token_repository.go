package repositories

import (
	"peerprep/user/internal/models"
	"time"

	"gorm.io/gorm"
)

type TokenRepository struct {
	DB *gorm.DB
}

func (r *TokenRepository) Create(token *models.Token) error {
	return r.DB.Create(token).Error
}

func (r *TokenRepository) GetByToken(tokenStr string) (*models.Token, error) {
	var t models.Token
	if err := r.DB.Where("token = ?", tokenStr).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TokenRepository) GetByUserAndPurpose(userID uint, purpose models.TokenPurpose) (*models.Token, error) {
	var t models.Token
	if err := r.DB.Where("user_id = ? AND purpose = ?", userID, purpose).Order("created_at DESC").First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TokenRepository) DeleteByID(id uint) error {
	return r.DB.Delete(&models.Token{}, id).Error
}

func (r *TokenRepository) DeleteByToken(tokenStr string) error {
	return r.DB.Where("token = ?", tokenStr).Delete(&models.Token{}).Error
}

func (r *TokenRepository) DeleteByUserAndPurpose(userID uint, purpose models.TokenPurpose) error {
	return r.DB.Where("user_id = ? AND purpose = ?", userID, purpose).Delete(&models.Token{}).Error
}

func (r *TokenRepository) DeleteExpired(before time.Time) (int64, error) {
	tx := r.DB.Where("expires_at <= ?", before).Delete(&models.Token{})
	return tx.RowsAffected, tx.Error
}
