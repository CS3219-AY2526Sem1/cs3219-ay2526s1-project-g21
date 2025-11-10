package gemini

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"google.golang.org/genai"
	"gorm.io/gorm"

	"peerprep/ai/internal/llm"
	"peerprep/ai/internal/models"
)

// Client represents a Gemini LLM client

type Client struct {
	client *genai.Client
	config *Config
	db     *gorm.DB // For querying active fine-tuned models
}

func NewClient(config *Config) (*Client, error) {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  config.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, &llm.ProviderError{
			Provider: "gemini",
			Code:     llm.ErrCodeAPIKey,
			Message:  "Failed to create Gemini client",
			Err:      err,
		}
	}

	return &Client{
		client: client,
		config: config,
		db:     nil, // Set later via SetDatabase if needed
	}, nil
}

// SetDatabase sets the database connection for model selection
func (c *Client) SetDatabase(db *gorm.DB) {
	c.db = db
}

// selectModel performs weighted random selection between base model and fine-tuned models
func (c *Client) selectModel(ctx context.Context) (string, string) {
	// If no database connection, use base model
	if c.db == nil {
		return c.config.Model, ""
	}

	// Get all active fine-tuned models with their traffic weights
	var activeModels []models.ModelVersion
	err := c.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("traffic_weight DESC").
		Find(&activeModels).Error

	if err != nil || len(activeModels) == 0 {
		// No fine-tuned models active, use base model
		return c.config.Model, ""
	}

	// Calculate total traffic weight for fine-tuned models
	totalFineTunedWeight := 0
	for _, m := range activeModels {
		totalFineTunedWeight += m.TrafficWeight
	}

	// Base model gets remaining weight (100 - sum of fine-tuned weights)
	baseWeight := 100 - totalFineTunedWeight
	if baseWeight < 0 {
		baseWeight = 0
	}

	// Weighted random selection
	random := rand.Intn(100)

	if random < baseWeight {
		// Use base model
		return c.config.Model, ""
	}

	// Route to fine-tuned model based on weight
	cumulative := baseWeight
	for _, m := range activeModels {
		cumulative += m.TrafficWeight
		if random < cumulative {
			// Use this fine-tuned model
			return m.VersionName, m.VersionName
		}
	}

	// Fallback to base model
	return c.config.Model, ""
}

// generates AI content based on the provided prompt
func (c *Client) GenerateContent(ctx context.Context, prompt string, requestID string, detailLevel string) (*models.GenerationResponse, error) {
	startTime := time.Now()

	// Select model based on traffic weights (A/B testing)
	selectedModel, modelVersion := c.selectModel(ctx)

	result, err := c.client.Models.GenerateContent(
		ctx,
		selectedModel,
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		code := llm.ErrCodeServiceDown
		message := "Failed to generate content"

		// Detect rate limit errors
		if isRateLimitError(err) {
			code = llm.ErrCodeRateLimit
			message = "Rate limit exceeded, please try again later"
		}

		return nil, &llm.ProviderError{
			Provider: "gemini",
			Code:     code,
			Message:  message,
			Err:      err,
		}
	}

	// Extract the response text
	if result == nil {
		return nil, &llm.ProviderError{
			Provider: "gemini",
			Code:     llm.ErrCodeInvalidInput,
			Message:  "No response generated",
		}
	}

	content, err := result.Text()
	if err != nil {
		return nil, &llm.ProviderError{
			Provider: "gemini",
			Code:     llm.ErrCodeInvalidInput,
			Message:  "Failed to extract response text",
			Err:      err,
		}
	}
	if content == "" {
		return nil, &llm.ProviderError{
			Provider: "gemini",
			Code:     llm.ErrCodeInvalidInput,
			Message:  "Empty response generated",
		}
	}

	processingTime := time.Since(startTime).Milliseconds()

	return &models.GenerationResponse{
		Content:   content,
		RequestID: requestID,
		Metadata: models.GenerationMetadata{
			ProcessingTime: int(processingTime),
			DetailLevel:    detailLevel,
			Provider:       "gemini",
			Model:          selectedModel,
			ModelVersion:   modelVersion,
		},
	}, nil
}

func (c *Client) GetProviderName() string {
	return "gemini"
}

// checks if the error is a rate limit error from Gemini API
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "resource_exhausted") ||
		strings.Contains(errStr, "quota exceeded") ||
		strings.Contains(errStr, "rate limit")
}
