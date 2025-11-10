package tuning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"peerprep/ai/internal/models"

	"gorm.io/gorm"
)

// GeminiTuner handles fine-tuning operations with Vertex AI API
type GeminiTuner struct {
	projectID string
	region    string
	db        *gorm.DB
}

// TuningConfig contains configuration for fine-tuning jobs
type TuningConfig struct {
	BaseModel        string
	TrainingFilePath string // GCS URI like gs://bucket/path/to/training.jsonl
	LearningRate     float64
	EpochCount       int
	AdapterSize      string // "ADAPTER_SIZE_ONE", "ADAPTER_SIZE_FOUR", "ADAPTER_SIZE_EIGHT", "ADAPTER_SIZE_SIXTEEN"
}

// VertexAITuningRequest represents the request body for creating a tuning job
type VertexAITuningRequest struct {
	BaseModel             string               `json:"baseModel"`
	SupervisedTuningSpec  SupervisedTuningSpec `json:"supervisedTuningSpec"`
	TunedModelDisplayName string               `json:"tunedModelDisplayName,omitempty"`
}

type SupervisedTuningSpec struct {
	TrainingDatasetURI string           `json:"trainingDatasetUri"`
	HyperParameters    *HyperParameters `json:"hyperParameters,omitempty"`
}

type HyperParameters struct {
	EpochCount             int     `json:"epochCount,omitempty"`
	LearningRateMultiplier float64 `json:"learningRateMultiplier,omitempty"`
	AdapterSize            string  `json:"adapterSize,omitempty"`
}

// VertexAITuningResponse represents the response from creating a tuning job
type VertexAITuningResponse struct {
	Name                  string          `json:"name"`
	BaseModel             string          `json:"baseModel"`
	TunedModelDisplayName string          `json:"tunedModelDisplayName"`
	State                 string          `json:"state"`
	CreateTime            time.Time       `json:"createTime"`
	TunedModel            *TunedModelInfo `json:"tunedModel,omitempty"`
}

// TunedModelInfo contains information about the deployed tuned model
type TunedModelInfo struct {
	Model    string `json:"model,omitempty"`    // e.g., "tunedModels/3365709546226974720"
	Endpoint string `json:"endpoint,omitempty"` // Full endpoint path
}

// NewGeminiTuner creates a new Gemini tuner using Vertex AI REST API
func NewGeminiTuner(apiKey string, projectID string, db *gorm.DB) (*GeminiTuner, error) {
	// For Vertex AI, we use gcloud auth instead of API key
	// The region defaults to us-central1 but can be configured via env
	region := "us-central1"

	return &GeminiTuner{
		projectID: projectID,
		region:    region,
		db:        db,
	}, nil
}

// getAccessToken gets OAuth2 access token using gcloud CLI
func (gt *GeminiTuner) getAccessToken() (string, error) {
	cmd := exec.Command("gcloud", "auth", "print-access-token")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get access token (ensure gcloud CLI is installed and authenticated): %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// CreateTuningJob starts a new fine-tuning job with Vertex AI
func (gt *GeminiTuner) CreateTuningJob(ctx context.Context, config *TuningConfig) (*models.ModelVersion, error) {
	log.Printf("Starting Vertex AI fine-tuning job: base_model=%s, training_file=%s", config.BaseModel, config.TrainingFilePath)

	// Get access token
	token, err := gt.getAccessToken()
	if err != nil {
		return nil, err
	}

	// Prepare request body
	requestBody := VertexAITuningRequest{
		BaseModel: config.BaseModel,
		SupervisedTuningSpec: SupervisedTuningSpec{
			TrainingDatasetURI: config.TrainingFilePath, // Must be GCS URI
			HyperParameters: &HyperParameters{
				EpochCount:             config.EpochCount,
				LearningRateMultiplier: config.LearningRate,
				AdapterSize:            config.AdapterSize,
			},
		},
		TunedModelDisplayName: fmt.Sprintf("peerprep-tuned-%d", time.Now().Unix()),
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create tuning job via REST API
	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/tuningJobs",
		gt.region, gt.projectID, gt.region)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("tuning job creation failed (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tuningResp VertexAITuningResponse
	if err := json.Unmarshal(body, &tuningResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("Tuning job created: job_id=%s, state=%s", tuningResp.Name, tuningResp.State)

	// Count training samples from file path
	trainingDataSize := 0 // We can't count from GCS URI easily, leave as 0

	// Create model version record
	modelVersion := &models.ModelVersion{
		VersionName:      tuningResp.TunedModelDisplayName,
		BaseModel:        config.BaseModel,
		TrainingJobID:    tuningResp.Name,
		TrainingDataSize: trainingDataSize,
		IsActive:         false,
		TrafficWeight:    0,
	}

	if err := gt.db.Create(modelVersion).Error; err != nil {
		return nil, fmt.Errorf("failed to store model version: %w", err)
	}

	return modelVersion, nil
}

// GetTuningJobStatus checks the status of a tuning job
func (gt *GeminiTuner) GetTuningJobStatus(ctx context.Context, jobID string) (string, error) {
	token, err := gt.getAccessToken()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/%s", gt.region, jobID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get tuning job status (status %d): %s", resp.StatusCode, string(body))
	}

	var tuningResp VertexAITuningResponse
	if err := json.Unmarshal(body, &tuningResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Map Vertex AI states to simple status strings
	// Possible states: "JOB_STATE_PENDING", "JOB_STATE_RUNNING", "JOB_STATE_SUCCEEDED", "JOB_STATE_FAILED", "JOB_STATE_CANCELLED"
	switch tuningResp.State {
	case "JOB_STATE_PENDING", "JOB_STATE_RUNNING":
		return "creating", nil
	case "JOB_STATE_SUCCEEDED":
		return "completed", nil
	case "JOB_STATE_FAILED", "JOB_STATE_CANCELLED":
		return "failed", nil
	default:
		return "unknown", nil
	}
}

// WaitForCompletion waits for a tuning job to complete and updates the model with endpoint
func (gt *GeminiTuner) WaitForCompletion(ctx context.Context, jobID string, maxWait time.Duration) error {
	log.Printf("Waiting for tuning job to complete: job_id=%s", jobID)

	timeout := time.After(maxWait)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("tuning job timed out after %v", maxWait)
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := gt.GetTuningJobStatus(ctx, jobID)
			if err != nil {
				return err
			}

			log.Printf("Tuning job status: %s", status)

			if status == "completed" {
				// Fetch full job details to get tuned model endpoint
				if err := gt.UpdateModelEndpoint(ctx, jobID); err != nil {
					log.Printf("Warning: Could not update model endpoint: %v", err)
				}
				return nil
			}
			if status == "failed" {
				return fmt.Errorf("tuning job failed")
			}
		}
	}
}

// UpdateModelEndpoint fetches the completed tuning job and updates the model with the endpoint
func (gt *GeminiTuner) UpdateModelEndpoint(ctx context.Context, jobID string) error {
	token, err := gt.getAccessToken()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/%s", gt.region, jobID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get tuning job details (status %d): %s", resp.StatusCode, string(body))
	}

	var tuningResp VertexAITuningResponse
	if err := json.Unmarshal(body, &tuningResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract model reference from tunedModel.model
	if tuningResp.TunedModel != nil && tuningResp.TunedModel.Model != "" {
		modelRef := tuningResp.TunedModel.Model // e.g., "tunedModels/3365709546226974720"

		// Update the model_versions record with the correct model reference
		result := gt.db.Model(&models.ModelVersion{}).
			Where("training_job_id = ?", jobID).
			Update("version_name", modelRef)

		if result.Error != nil {
			return fmt.Errorf("failed to update model endpoint: %w", result.Error)
		}

		log.Printf("Updated model version with endpoint: %s", modelRef)
	}

	return nil
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
			"is_active":      true,
			"traffic_weight": trafficWeight,
			"activated_at":   now,
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
