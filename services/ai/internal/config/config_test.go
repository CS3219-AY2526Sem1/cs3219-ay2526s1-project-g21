package config

import "testing"

func TestLoadConfig_DefaultProvider(t *testing.T) {
	t.Setenv("AI_PROVIDER", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.Provider != "gemini" {
		t.Fatalf("expected provider gemini, got %s", cfg.Provider)
	}
}

func TestLoadConfig_UnsupportedProvider(t *testing.T) {
	t.Setenv("AI_PROVIDER", "unknown")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	t.Setenv("UNIT_TEST_ENV", "value")
	if got := getEnvOrDefault("UNIT_TEST_ENV", "fallback"); got != "value" {
		t.Fatalf("expected env value, got %s", got)
	}

	t.Setenv("UNIT_TEST_ENV", "")
	if got := getEnvOrDefault("UNIT_TEST_ENV", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback value, got %s", got)
	}
}
