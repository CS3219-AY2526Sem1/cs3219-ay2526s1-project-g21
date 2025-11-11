package gemini

import "testing"

func TestNewConfig(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "key")
	t.Setenv("GEMINI_MODEL", "custom")

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("NewConfig returned error: %v", err)
	}

	if cfg.APIKey != "key" || cfg.Model != "custom" {
		t.Fatalf("unexpected config values: %+v", cfg)
	}
}

func TestNewConfigMissingKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	if _, err := NewConfig(); err == nil {
		t.Fatal("expected error when API key missing")
	}
}
