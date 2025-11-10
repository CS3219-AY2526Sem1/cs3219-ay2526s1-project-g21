package tuning

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/genai"

	"peerprep/ai/internal/models"

	"gorm.io/gorm"
)

// GeminiTuner handles fine-tuning operations with Gemini API
type GeminiTuner struct {
	client    *genai.Client
	projectID string
	db        *gorm.DB
}

// TuningConfig contains configuration for fine-tuning jobs
type TuningConfig struct {
	BaseModel        string
	TrainingFilePath string
	LearningRate     float64
	EpochCount       int
	BatchSize        int
}

// NewGeminiTuner creates a new Gemini tuner
func NewGeminiTuner(apiKey string, projectID string, db *gorm.DB) (*GeminiTuner, error) {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &GeminiTuner{
		client:    client,
		projectID: projectID,
		db:        db,
	}, nil
}

// CreateTuningJob starts a new fine-tuning job with Gemini
// NOTE: Gemini Fine-Tuning API is not yet available in the Go SDK
// This is a placeholder for future implementation
func (gt *GeminiTuner) CreateTuningJob(ctx context.Context, config *TuningConfig) (*models.ModelVersion, error) {
	log.Printf("Starting fine-tuning job: base_model=%s, training_file=%s", config.BaseModel, config.TrainingFilePath)

	// TODO: Implement when Gemini Go SDK adds fine-tuning support
	// For now, return an error indicating the feature is not available
	return nil, fmt.Errorf("gemini fine-tuning API is not yet available in the Go SDK - please use the REST API or Python SDK instead")

	// When the API becomes available, the implementation will be:
	// 1. Upload training data file to Gemini
	// 2. Create tuning job with hyperparameters
	// 3. Store model version in database
	// 4. Return model version

	/*
		// Upload training data file
		file, err := os.Open(config.TrainingFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open training file: %w", err)
		}
		defer file.Close()

		// Count training samples from file
		trainingDataSize, err := countJSONLLines(file)
		if err != nil {
			trainingDataSize = 0
		}

		// Create model version record
		modelVersion := &models.ModelVersion{
			VersionName:      fmt.Sprintf("%s-ft-%d", config.BaseModel, time.Now().Unix()),
			BaseModel:        config.BaseModel,
			TrainingJobID:    "placeholder",
			TrainingDataSize: trainingDataSize,
			IsActive:         false,
			TrafficWeight:    0,
		}

		if err := gt.db.Create(modelVersion).Error; err != nil {
			return nil, fmt.Errorf("failed to store model version: %w", err)
		}

		return modelVersion, nil
	*/
}

// GetTuningJobStatus checks the status of a tuning job
// NOTE: Placeholder until Gemini Fine-Tuning API is available
func (gt *GeminiTuner) GetTuningJobStatus(ctx context.Context, jobID string) (string, error) {
	return "", fmt.Errorf("gemini fine-tuning API is not yet available in the Go SDK")
}

// WaitForCompletion waits for a tuning job to complete
// NOTE: Placeholder until Gemini Fine-Tuning API is available
func (gt *GeminiTuner) WaitForCompletion(ctx context.Context, jobID string, maxWait time.Duration) error {
	return fmt.Errorf("gemini fine-tuning API is not yet available in the Go SDK")
}

// ActivateModel activates a fine-tuned model with specified traffic weight
func (gt *GeminiTuner) ActivateModel(modelVersionID uint, trafficWeight int) error {
	if trafficWeight < 0 || trafficWeight > 100 {
		return fmt.Errorf("traffic weight must be between 0 and 100")
	}

	now := time.Now()

	// Update model version
	result := gt.db.Model(&models.ModelVersion{}).
		Where("id = ?", modelVersionID).
		Updates(map[string]interface{}{
			"is_active":     true,
			"traffic_weight": trafficWeight,
			"activated_at":  now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to activate model: %w", result.Error)
	}

	log.Printf("Activated model version %d with %d%% traffic", modelVersionID, trafficWeight)

	return nil
}

// DeactivateModel deactivates a fine-tuned model
func (gt *GeminiTuner) DeactivateModel(modelVersionID uint) error {
	now := time.Now()

	result := gt.db.Model(&models.ModelVersion{}).
		Where("id = ?", modelVersionID).
		Updates(map[string]interface{}{
			"is_active":      false,
			"traffic_weight": 0,
			"deactivated_at": now,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to deactivate model: %w", result.Error)
	}

	log.Printf("Deactivated model version %d", modelVersionID)

	return nil
}

// GetActiveModel returns the currently active model with highest traffic weight
func (gt *GeminiTuner) GetActiveModel() (*models.ModelVersion, error) {
	var modelVersion models.ModelVersion

	err := gt.db.Where("is_active = ?", true).
		Order("traffic_weight DESC").
		First(&modelVersion).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No active model
		}
		return nil, fmt.Errorf("failed to get active model: %w", err)
	}

	return &modelVersion, nil
}

// countJSONLLines counts the number of lines in a JSONL file
func countJSONLLines(reader io.Reader) (int, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}

	// Account for last line without newline
	if len(data) > 0 && data[len(data)-1] != '\n' {
		count++
	}

	return count, nil
}
