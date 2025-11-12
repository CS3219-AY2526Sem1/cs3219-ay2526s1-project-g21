package gemini

import (
	"errors"
	"os"
)

// holds Gemini-specific configuration
type Config struct {
	APIKey   string
	Model    string
	Project  string
	Location string
}

func NewConfig() (*Config, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("GEMINI_API_KEY environment variable is required")
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash" // default model
	}

	project := os.Getenv("GEMINI_PROJECT_ID")
	location := os.Getenv("GEMINI_LOCATION")
	if location == "" {
		location = "us-central1" // default location
	}

	return &Config{
		APIKey:   apiKey,
		Model:    model,
		Project:  project,
		Location: location,
	}, nil
}
