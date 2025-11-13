package gemini

import (
	"context"
	"log"
	"math/rand"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	"google.golang.org/genai"
	"gorm.io/gorm"

	"peerprep/ai/internal/llm"
	"peerprep/ai/internal/models"
)

// Client represents a Gemini LLM client

type Client struct {
	apiKeyClient *genai.Client // For base models with API key
	vertexClient *genai.Client // For Vertex AI endpoints with OAuth2
	config       *Config
	db           *gorm.DB // For querying active fine-tuned models
}

func NewClient(config *Config) (*Client, error) {
	ctx := context.Background()

	// Create API key client for base models
	apiKeyClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  config.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, &llm.ProviderError{
			Provider: "gemini",
			Code:     llm.ErrCodeAPIKey,
			Message:  "Failed to create Gemini API client",
			Err:      err,
		}
	}

	// Try to create Vertex AI client with OAuth2 for fine-tuned endpoints
	var vertexClient *genai.Client
	_, err = google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err == nil && config.Project != "" {
		// OAuth2 credentials available and project configured
		log.Printf("[Init] OAuth2 credentials found, creating Vertex AI client for project %s in %s...", config.Project, config.Location)
		vertexClient, err = genai.NewClient(ctx, &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  config.Project,
			Location: config.Location,
		})
		if err != nil {
			log.Printf("[Init] Failed to create Vertex AI client: %v", err)
		} else {
			log.Printf("[Init] Vertex AI client created successfully")
		}
	} else {
		if err != nil {
			log.Printf("[Init] No OAuth2 credentials available: %v", err)
		} else {
			log.Printf("[Init] Project not configured, skipping Vertex AI client")
		}
	}
	// If OAuth2 fails, vertexClient will be nil and we'll skip fine-tuned models

	return &Client{
		apiKeyClient: apiKeyClient,
		vertexClient: vertexClient,
		config:       config,
		db:           nil, // Set later via SetDatabase if needed
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

	// Determine which client to use
	var client *genai.Client
	isVertexEndpoint := strings.HasPrefix(selectedModel, "projects/")

	if isVertexEndpoint {
		// Use Vertex AI client for fine-tuned endpoints
		if c.vertexClient == nil {
			// No OAuth2 credentials, fall back to base model
			log.Printf("[Model Selection] Vertex AI endpoint selected but no OAuth2 credentials available. Falling back to base model: %s (Request: %s)", c.config.Model, requestID)
			selectedModel = c.config.Model
			modelVersion = ""
			client = c.apiKeyClient
		} else {
			log.Printf("[Model Selection] Using Vertex AI fine-tuned endpoint: %s (Request: %s)", selectedModel, requestID)
			client = c.vertexClient
		}
	} else {
		// Use API key client for base models
		log.Printf("[Model Selection] Using base model with API key: %s (Request: %s)", selectedModel, requestID)
		client = c.apiKeyClient
	}

	result, err := client.Models.GenerateContent(
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
