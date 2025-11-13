package llm

import (
	"context"
	"peerprep/ai/internal/models"
)

// defines the interface for LLM providers
type Provider interface {
	GenerateContent(ctx context.Context, prompt string, requestID string, detailLevel string) (*models.GenerationResponse, error)
	GetProviderName() string
}

// represents an error from an LLM provider
type ProviderError struct {
	Provider string
	Code     string
	Message  string
	Err      error
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return e.Provider + " error: " + e.Message + " (" + e.Err.Error() + ")"
	}
	return e.Provider + " error: " + e.Message
}

// Common error codes
// For current and future use across different providers
const (
	ErrCodeAPIKey       = "invalid_api_key"
	ErrCodeRateLimit    = "rate_limit_exceeded"
	ErrCodeServiceDown  = "service_unavailable"
	ErrCodeInvalidInput = "invalid_input"
	ErrCodeTimeout      = "timeout"
)
