package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"peerprep/ai/internal/config"
	"testing"
	"text/template"
)

// ============================================================================
// Test Helpers
// ============================================================================

func newTestHealthHandler(provider *mockProvider, promptMgr *mockPromptManager, cfg *config.Config) *HealthHandler {
	handler := &HealthHandler{
		config: cfg,
	}

	if provider != nil {
		handler.provider = provider
	}
	if promptMgr != nil {
		handler.promptManager = promptMgr
	}

	return handler
}

func decodeReadinessResponse(t *testing.T, rec *httptest.ResponseRecorder) ReadinessResponse {
	t.Helper()
	var response ReadinessResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	return response
}

// ============================================================================
// ReadyzHandler Tests
// ============================================================================

func TestReadyzHandler_AllHealthy(t *testing.T) {
	provider := &mockProvider{}
	promptMgr := &mockPromptManager{}
	cfg := &config.Config{Provider: "gemini"}
	handler := newTestHealthHandler(provider, promptMgr, cfg)

	req := httptest.NewRequest("GET", "/readyz", nil)
	rec := httptest.NewRecorder()

	// Execute
	handler.ReadyzHandler(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	response := decodeReadinessResponse(t, rec)

	if response.Status != "ready" {
		t.Errorf("expected status 'ready', got '%s'", response.Status)
	}

	if response.Service != "ai" {
		t.Errorf("expected service 'ai', got '%s'", response.Service)
	}

	expectedChecks := []string{"provider", "prompt_manager", "configuration"}
	for _, checkName := range expectedChecks {
		check, exists := response.Checks[checkName]
		if !exists {
			t.Errorf("missing check: %s", checkName)
			continue
		}
		if check.Status != "ok" {
			t.Errorf("check %s: expected status 'ok', got '%s'", checkName, check.Status)
		}
	}
}

func TestReadyzHandler_DependenciesFail(t *testing.T) {
	// Setup - all dependencies nil
	handler := newTestHealthHandler(nil, nil, nil)

	req := httptest.NewRequest("GET", "/readyz", nil)
	rec := httptest.NewRecorder()

	// Execute
	handler.ReadyzHandler(rec, req)

	// Assert
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}

	response := decodeReadinessResponse(t, rec)

	if response.Status != "not_ready" {
		t.Errorf("expected status 'not_ready', got '%s'", response.Status)
	}

	// Verify all checks fail
	expectedFailures := []string{"provider", "prompt_manager", "configuration"}
	for _, checkName := range expectedFailures {
		check, exists := response.Checks[checkName]
		if !exists {
			t.Errorf("missing check: %s", checkName)
			continue
		}
		if check.Status != "failed" {
			t.Errorf("check %s: expected status 'failed', got '%s'", checkName, check.Status)
		}
		if check.Message == "" {
			t.Errorf("check %s: expected error message, got empty string", checkName)
		}
	}
}

func TestReadyzHandler_NoTemplatesLoaded(t *testing.T) {
	// Setup - prompt manager with empty templates
	provider := &mockProvider{}
	promptMgr := &mockPromptManager{
		getTemplatesFn: func() map[string]map[string]*template.Template {
			return map[string]map[string]*template.Template{}
		},
	}
	cfg := &config.Config{Provider: "gemini"}
	handler := newTestHealthHandler(provider, promptMgr, cfg)

	req := httptest.NewRequest("GET", "/readyz", nil)
	rec := httptest.NewRecorder()

	// Execute
	handler.ReadyzHandler(rec, req)

	// Assert
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", rec.Code)
	}

	response := decodeReadinessResponse(t, rec)

	if response.Status != "not_ready" {
		t.Errorf("expected status 'not_ready', got '%s'", response.Status)
	}

	// Verify prompt_manager check fails
	pmCheck, exists := response.Checks["prompt_manager"]
	if !exists {
		t.Fatal("prompt_manager check missing from response")
	}
	if pmCheck.Status != "failed" {
		t.Errorf("expected prompt_manager check status 'failed', got '%s'", pmCheck.Status)
	}
	if pmCheck.Message != "No prompt templates loaded" {
		t.Errorf("expected error message about no templates, got '%s'", pmCheck.Message)
	}
}

// ============================================================================
// HealthzHandler Tests
// ============================================================================

func TestHealthzHandler_AlwaysReturnsOK(t *testing.T) {
	// Setup - even with nil dependencies, healthz should work (liveness probe)
	handler := newTestHealthHandler(nil, nil, nil)

	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	// Execute
	handler.HealthzHandler(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%s'", response["status"])
	}
	if response["service"] != "ai" {
		t.Errorf("expected service 'ai', got '%s'", response["service"])
	}
	if response["version"] != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", response["version"])
	}
}
