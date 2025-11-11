package feedback

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"peerprep/ai/internal/models"

	"gorm.io/gorm"
)

// FeedbackManager handles AI feedback storage and export
type FeedbackManager struct {
	db           *gorm.DB
	contextCache *ContextCache
}

// NewFeedbackManager creates a new feedback manager
func NewFeedbackManager(db *gorm.DB, cacheTTL time.Duration) *FeedbackManager {
	return &FeedbackManager{
		db:           db,
		contextCache: NewContextCache(cacheTTL),
	}
}

// StoreRequestContext caches request context for later feedback
func (fm *FeedbackManager) StoreRequestContext(ctx *models.RequestContext) {
	fm.contextCache.Set(ctx.RequestID, ctx)
	log.Printf("Stored request context: %s (type: %s)", ctx.RequestID, ctx.RequestType)
}

// SubmitFeedback stores user feedback for a request
func (fm *FeedbackManager) SubmitFeedback(requestID string, isPositive bool) error {
	// Get context from cache
	ctx, exists := fm.contextCache.Get(requestID)
	if !exists {
		return fmt.Errorf("request context not found or expired: %s", requestID)
	}

	// Create feedback record
	feedback := &models.AIFeedback{
		RequestID:    requestID,
		RequestType:  ctx.RequestType,
		Prompt:       ctx.Prompt,
		Response:     ctx.Response,
		IsPositive:   isPositive,
		ModelVersion: ctx.ModelVersion,
		FeedbackAt:   time.Now(),
		Exported:     false,
	}

	// Store in database
	if err := fm.db.Create(feedback).Error; err != nil {
		return fmt.Errorf("failed to store feedback: %w", err)
	}

	// Remove from cache after successful storage
	fm.contextCache.Delete(requestID)

	log.Printf("Stored feedback: request=%s, positive=%v, type=%s", requestID, isPositive, ctx.RequestType)

	return nil
}

// GetUnexportedFeedback retrieves feedback that hasn't been exported yet
func (fm *FeedbackManager) GetUnexportedFeedback(limit int) ([]models.AIFeedback, error) {
	var feedback []models.AIFeedback

	query := fm.db.Where("exported = ?", false).Order("feedback_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&feedback).Error; err != nil {
		return nil, fmt.Errorf("failed to get unexported feedback: %w", err)
	}

	return feedback, nil
}

// GetFeedbackSince retrieves feedback since a specific time
func (fm *FeedbackManager) GetFeedbackSince(since time.Time, limit int) ([]models.AIFeedback, error) {
	var feedback []models.AIFeedback

	query := fm.db.Where("feedback_at >= ?", since).Order("feedback_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&feedback).Error; err != nil {
		return nil, fmt.Errorf("failed to get feedback since %v: %w", since, err)
	}

	return feedback, nil
}

// MarkAsExported marks feedback records as exported
func (fm *FeedbackManager) MarkAsExported(feedbackIDs []uint) error {
	now := time.Now()
	result := fm.db.Model(&models.AIFeedback{}).
		Where("id IN ?", feedbackIDs).
		Updates(map[string]interface{}{
			"exported":    true,
			"exported_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to mark feedback as exported: %w", result.Error)
	}

	log.Printf("Marked %d feedback records as exported", result.RowsAffected)
	return nil
}

// ExportToJSONL exports feedback to JSONL format for Gemini fine-tuning
// Only exports positive feedback (thumbs up) as training examples
func (fm *FeedbackManager) ExportToJSONL(feedback []models.AIFeedback) ([]byte, error) {
	var jsonlLines []string

	for _, fb := range feedback {
		// Only export positive feedback for fine-tuning
		if !fb.IsPositive {
			continue
		}

		// Create training data point in Gemini format
		dataPoint := models.TrainingDataPoint{
			Contents: []models.TrainingContent{
				{
					Role: "user",
					Parts: []models.TrainingPart{
						{Text: fb.Prompt},
					},
				},
				{
					Role: "model",
					Parts: []models.TrainingPart{
						{Text: fb.Response},
					},
				},
			},
		}

		// Marshal to JSON
		jsonBytes, err := json.Marshal(dataPoint)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal training data: %w", err)
		}

		jsonlLines = append(jsonlLines, string(jsonBytes))
	}

	// Join with newlines to create JSONL
	jsonlData := []byte{}
	for i, line := range jsonlLines {
		jsonlData = append(jsonlData, []byte(line)...)
		if i < len(jsonlLines)-1 {
			jsonlData = append(jsonlData, '\n')
		}
	}

	log.Printf("Exported %d positive feedback examples to JSONL (%d total feedback records)", len(jsonlLines), len(feedback))

	return jsonlData, nil
}

// GetFeedbackStats returns statistics about stored feedback
func (fm *FeedbackManager) GetFeedbackStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total feedback count
	var totalCount int64
	if err := fm.db.Model(&models.AIFeedback{}).Count(&totalCount).Error; err != nil {
		return nil, err
	}
	stats["total_count"] = totalCount

	// Positive feedback count
	var positiveCount int64
	if err := fm.db.Model(&models.AIFeedback{}).Where("is_positive = ?", true).Count(&positiveCount).Error; err != nil {
		return nil, err
	}
	stats["positive_count"] = positiveCount

	// Unexported count
	var unexportedCount int64
	if err := fm.db.Model(&models.AIFeedback{}).Where("exported = ?", false).Count(&unexportedCount).Error; err != nil {
		return nil, err
	}
	stats["unexported_count"] = unexportedCount

	// Cached contexts
	stats["cached_contexts"] = fm.contextCache.Size()

	return stats, nil
}
