package config

import (
	"errors"
	"os"
)

// app config, mostly AI provider related
type Config struct {
	Provider string
}

// loads configuration from environment variables
func LoadConfig() (*Config, error) {
	config := &Config{
		Provider: getEnvOrDefault("AI_PROVIDER", "gemini"),
	}
	if err := validateConfig(config); err != nil {
		return nil, err
	}
	return config, nil
}

func validateConfig(config *Config) error {
	if config.Provider != "gemini" {
		return errors.New("unsupported AI provider: " + config.Provider + ". Currently supported: gemini")
	}
	// Gemini validation is handled by gemini.NewConfig()
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
