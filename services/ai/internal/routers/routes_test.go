package routers

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

type stubProvider struct{}

func (stubProvider) GenerateContent(context.Context, string, string, string) (*models.GenerationResponse, error) {
	return &models.GenerationResponse{}, nil
}

func (stubProvider) GetProviderName() string { return "stub" }

type stubPromptManager struct{}

func (stubPromptManager) BuildPrompt(string, string, interface{}) (string, error) {
	return "prompt", nil
}

func (stubPromptManager) GetTemplates() map[string]map[string]*template.Template {
	return map[string]map[string]*template.Template{}
}

var (
	_ llm.Provider           = (*stubProvider)(nil)
	_ prompts.PromptProvider = (*stubPromptManager)(nil)
)

func TestHealthRoutes(t *testing.T) {
	router := chi.NewRouter()
	handler := handlers.NewHealthHandler(nil, nil, &config.Config{Provider: "gemini"})

	HealthRoutes(router, handler)

	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz route not registered correctly, got status %d", rec.Code)
	}
}

func TestAIRoutesRegistersEndpoints(t *testing.T) {
	router := chi.NewRouter()
	logger := zap.NewNop()
	aiHandler := handlers.NewAIHandler(stubProvider{}, stubPromptManager{}, logger)
	feedbackHandler := handlers.NewFeedbackHandler(nil)
	modelHandler := handlers.NewModelHandler(nil, nil)

	AIRoutes(router, aiHandler, feedbackHandler, modelHandler)

	paths := map[string]bool{}
	if err := chi.Walk(router, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		paths[method+" "+route] = true
		return nil
	}); err != nil {
		t.Fatalf("failed walking routes: %v", err)
	}

	expected := []string{
		"GET /api/v1/ai/feedback/export",
		"GET /api/v1/ai/feedback/stats",
		"POST /api/v1/ai/explain",
		"POST /api/v1/ai/hint",
		"POST /api/v1/ai/tests",
		"POST /api/v1/ai/refactor-tips",
		"POST /api/v1/ai/feedback/{request_id}",
		"GET /api/v1/ai/models",
		"GET /api/v1/ai/models/{model_id}/stats",
		"PUT /api/v1/ai/models/{model_id}/traffic",
		"PUT /api/v1/ai/models/{model_id}/deactivate",
	}

	for _, route := range expected {
		if !paths[route] {
			t.Fatalf("expected route %s to be registered", route)
		}
	}
}
