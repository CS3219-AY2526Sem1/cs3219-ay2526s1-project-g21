package repositories

import (
	"errors"
	"peerprep/user/internal/models"

	"gorm.io/gorm"
)

type HistoryRepository struct {
	DB *gorm.DB
}

// Create creates a new interview history record
func (r *HistoryRepository) Create(history *models.InterviewHistory) error {
	var existing models.InterviewHistory

	err := r.DB.
		Where("match_id = ? AND (user1_id = ? OR user2_id = ?)",
			history.MatchID, history.User1ID, history.User2ID).
		First(&existing).Error

	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err // actual DB error
	}

	return r.DB.Create(history).Error
}

// GetByUserID retrieves all interview history for a specific user (either as user1 or user2)
func (r *HistoryRepository) GetByUserID(userID string) ([]models.InterviewHistory, error) {
	histories := []models.InterviewHistory{}

	// Use a portable subquery to pick the latest record per match_id
	subQuery := r.DB.Model(&models.InterviewHistory{}).
		Select("MAX(id) as id").
		Where("user1_id = ? OR user2_id = ?", userID, userID).
		Group("match_id")

	err := r.DB.Where("id IN (?)", subQuery).
		Order("ended_at DESC").
		Find(&histories).Error
	return histories, err
}

// GetByMatchID retrieves a specific interview session by match ID
func (r *HistoryRepository) GetByMatchID(matchID string) (*models.InterviewHistory, error) {
	var history models.InterviewHistory
	err := r.DB.
		Where("match_id = ?", matchID).
		Order("ended_at DESC").
		First(&history).Error

	if err != nil {
		return nil, err
	}

	return &history, nil
}
