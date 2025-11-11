package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	"peerprep/ai/internal/config"
	"peerprep/ai/internal/handlers"
	"peerprep/ai/internal/llm"
	"peerprep/ai/internal/models"
	"peerprep/ai/internal/prompts"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type fakeProvider struct{}

func (fakeProvider) GenerateContent(context.Context, string, string, string) (*models.GenerationResponse, error) {
	return &models.GenerationResponse{}, nil
}
func (fakeProvider) GetProviderName() string { return "fake" }

type fakePrompt struct{}

func (fakePrompt) BuildPrompt(string, string, interface{}) (string, error) { return "prompt", nil }
func (fakePrompt) GetTemplates() map[string]map[string]*template.Template {
	return map[string]map[string]*template.Template{}
}

var (
	_ llm.Provider           = (*fakeProvider)(nil)
	_ prompts.PromptProvider = (*fakePrompt)(nil)
)

func TestEnvHelpers(t *testing.T) {
	t.Setenv("TEST_ENV", "value")
	if got := getEnv("TEST_ENV", "fallback"); got != "value" {
		t.Fatalf("getEnv returned %s", got)
	}
	if got := getEnv("MISSING_ENV", "fallback"); got != "fallback" {
		t.Fatalf("getEnv default failed, got %s", got)
	}

	t.Setenv("TEST_INT", "10")
	if got := getEnvInt("TEST_INT", 5); got != 10 {
		t.Fatalf("expected 10, got %d", got)
	}
	if got := getEnvInt("MISSING_INT", 7); got != 7 {
		t.Fatalf("expected default 7, got %d", got)
	}

	t.Setenv("TEST_FLOAT", "0.5")
	if got := getEnvFloat("TEST_FLOAT", 0.1); got != 0.5 {
		t.Fatalf("expected 0.5, got %f", got)
	}
	if got := getEnvFloat("MISSING_FLOAT", 0.3); got != 0.3 {
		t.Fatalf("expected default 0.3, got %f", got)
	}
}

func TestRegisterRoutes(t *testing.T) {
	router := chi.NewRouter()
	aiHandler := handlers.NewAIHandler(fakeProvider{}, fakePrompt{}, zap.NewNop())
	healthHandler := handlers.NewHealthHandler(nil, nil, &config.Config{Provider: "gemini"})

	registerRoutes(router, aiHandler, nil, nil, healthHandler)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected /healthz to be registered, got %d", rec.Code)
	}
}
