package gemini

import (
	"context"
	"time"

	"google.golang.org/genai"

	"peerprep/ai/internal/llm"
	"peerprep/ai/internal/models"
)

// Client represents a Gemini LLM client

type Client struct {
	client *genai.Client
	config *Config
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
	}, nil
}

// generates a code explanation
func (c *Client) GenerateExplanation(ctx context.Context, prompt string, requestID string, detailLevel string) (*models.ExplainResponse, error) {
	startTime := time.Now()
	result, err := c.client.Models.GenerateContent(
		ctx,
		c.config.Model,
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		return nil, &llm.ProviderError{
			Provider: "gemini",
			Code:     llm.ErrCodeServiceDown,
			Message:  "Failed to generate explanation",
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

	explanation, err := result.Text()
	if err != nil {
		return nil, &llm.ProviderError{
			Provider: "gemini",
			Code:     llm.ErrCodeInvalidInput,
			Message:  "Failed to extract response text",
			Err:      err,
		}
	}
	if explanation == "" {
		return nil, &llm.ProviderError{
			Provider: "gemini",
			Code:     llm.ErrCodeInvalidInput,
			Message:  "Empty response generated",
		}
	}

	processingTime := time.Since(startTime).Milliseconds()

	return &models.ExplainResponse{
		Explanation: explanation,
		RequestID:   requestID,
		Metadata: models.ExplanationMetadata{
			ProcessingTime: int(processingTime),
			DetailLevel:    detailLevel,
			Provider:       "gemini",
			Model:          c.config.Model,
		},
	}, nil
}

func (c *Client) GetProviderName() string {
	return "gemini"
}
