package repositories

import (
	"peerprep/user/internal/models"
	"time"

	"gorm.io/gorm"
)

// CleanupUnverifiedUserIfExpired deletes an unverified user if their verification token is expired.
// Returns true if a deletion occurred.
func CleanupUnverifiedUserIfExpired(userRepo *UserRepository, tokenRepo *TokenRepository, user *models.User) (bool, error) {
	if user.Verified {
		return false, nil
	}

	// Find a verification token for this user
	var t models.Token
	err := tokenRepo.DB.Where("user_id = ? AND purpose = ?", user.ID, models.TokenPurposeAccountVerification).First(&t).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// No token; treat as expired/invalid and delete the user to free up identifiers
			if delErr := userRepo.DB.Delete(&models.User{}, user.ID).Error; delErr != nil {
				return false, delErr
			}
			// Also ensure any tokens for this user are removed
			_ = tokenRepo.DeleteByUserAndPurpose(user.ID, models.TokenPurposeAccountVerification)
			return true, nil
		}
		return false, err
	}

	if time.Now().After(t.ExpiresAt) {
		// Token expired: delete user and token
		if delErr := userRepo.DB.Delete(&models.User{}, user.ID).Error; delErr != nil {
			return false, delErr
		}
		_ = tokenRepo.DeleteByID(t.ID)
		return true, nil
	}

	return false, nil
}
