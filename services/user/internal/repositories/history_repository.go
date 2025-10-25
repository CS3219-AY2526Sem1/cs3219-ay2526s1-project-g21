package repositories

import (
	"peerprep/user/internal/models"

	"gorm.io/gorm"
)

type HistoryRepository struct {
	DB *gorm.DB
}

// Create creates a new interview history record
func (r *HistoryRepository) Create(history *models.InterviewHistory) error {
	return r.DB.Create(history).Error
}

// GetByUserID retrieves all interview history for a specific user (either as user1 or user2)
func (r *HistoryRepository) GetByUserID(userID string) ([]models.InterviewHistory, error) {
	var histories []models.InterviewHistory
	err := r.DB.Where("user1_id = ? OR user2_id = ?", userID, userID).
		Order("ended_at DESC").
		Find(&histories).Error
	return histories, err
}

// GetByMatchID retrieves a specific interview session by match ID
func (r *HistoryRepository) GetByMatchID(matchID string) (*models.InterviewHistory, error) {
	var history models.InterviewHistory
	err := r.DB.Where("match_id = ?", matchID).First(&history).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}
