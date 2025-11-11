package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"peerprep/ai/internal/feedback"
	"peerprep/ai/internal/llm"
	"peerprep/ai/internal/middleware"
	"peerprep/ai/internal/models"
	"peerprep/ai/internal/prompts"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newSQLiteFeedbackManager(t *testing.T) *feedback.FeedbackManager {
	t.Helper()
	dsn := fmt.Sprintf("file:%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AIFeedback{}, &models.ModelVersion{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return feedback.NewFeedbackManager(db, time.Minute)
}

func newTestAIHandler(provider llm.Provider, prompt prompts.PromptProvider) *AIHandler {
	handler := NewAIHandler(provider, prompt, zap.NewNop())
	return handler
}

func performRequest(handler http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestEnsureRequestID(t *testing.T) {
	if ensureRequestID("custom") != "custom" {
		t.Fatalf("expected custom ID to be preserved")
	}
	if ensureRequestID("") == "" {
		t.Fatalf("expected new ID when input empty")
	}
}

func TestExplainHandlerSuccess(t *testing.T) {
	provider := &mockProvider{
		generateContentFn: func(ctx context.Context, prompt, requestID, detailLevel string) (*models.GenerationResponse, error) {
			return &models.GenerationResponse{
				Content: "answer",
				Metadata: models.GenerationMetadata{
					Model: "test-model",
				},
			}, nil
		},
	}
	promptMgr := &mockPromptManager{}

	handler := newTestAIHandler(provider, promptMgr)
	fm := newSQLiteFeedbackManager(t)
	handler.SetFeedbackManager(fm)

	wrapped := middleware.ValidateRequest[*models.ExplainRequest]()(http.HandlerFunc(handler.ExplainHandler))
	body := `{"code":"print(1)","language":"python","detail_level":"beginner"}`
	rec := performRequest(wrapped, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	stats, err := fm.GetFeedbackStats()
	if err != nil {
		t.Fatalf("GetFeedbackStats failed: %v", err)
	}
	if stats["cached_contexts"].(int) != 1 {
		t.Fatalf("expected context to be cached for feedback")
	}
}

func TestExplainHandlerPromptError(t *testing.T) {
	provider := &mockProvider{}
	promptMgr := &mockPromptManager{
		buildPromptFn: func(mode, variant string, data interface{}) (string, error) {
			return "", errors.New("boom")
		},
	}
	handler := newTestAIHandler(provider, promptMgr)

	wrapped := middleware.ValidateRequest[*models.ExplainRequest]()(http.HandlerFunc(handler.ExplainHandler))
	rec := performRequest(wrapped, `{"code":"print(1)","language":"python","detail_level":"beginner"}`)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestExplainHandlerRateLimit(t *testing.T) {
	provider := &mockProvider{
		generateContentFn: func(ctx context.Context, prompt, requestID, detailLevel string) (*models.GenerationResponse, error) {
			return nil, &llm.ProviderError{Code: llm.ErrCodeRateLimit}
		},
	}
	handler := newTestAIHandler(provider, &mockPromptManager{})

	wrapped := middleware.ValidateRequest[*models.ExplainRequest]()(http.HandlerFunc(handler.ExplainHandler))
	rec := performRequest(wrapped, `{"code":"print(1)","language":"python","detail_level":"beginner"}`)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
}

func TestHintHandlerSuccess(t *testing.T) {
	provider := &mockProvider{
		generateContentFn: func(ctx context.Context, prompt, requestID, detailLevel string) (*models.GenerationResponse, error) {
			return &models.GenerationResponse{
				Content: "here is a hint",
				Metadata: models.GenerationMetadata{
					ModelVersion: "v1",
				},
			}, nil
		},
	}
	handler := newTestAIHandler(provider, &mockPromptManager{})
	handler.SetFeedbackManager(newSQLiteFeedbackManager(t))

	wrapped := middleware.ValidateRequest[*models.HintRequest]()(http.HandlerFunc(handler.HintHandler))
	body := `{"code":"print(1)","language":"python","hint_level":"beginner","question":{"prompt_markdown":"desc"}}`
	rec := performRequest(wrapped, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTestsHandlerError(t *testing.T) {
	provider := &mockProvider{
		generateContentFn: func(ctx context.Context, prompt, requestID, detailLevel string) (*models.GenerationResponse, error) {
			return nil, errors.New("provider failed")
		},
	}
	handler := newTestAIHandler(provider, &mockPromptManager{})

	wrapped := middleware.ValidateRequest[*models.TestGenRequest]()(http.HandlerFunc(handler.TestsHandler))
	body := `{"code":"print(1)","language":"python","question":{"prompt_markdown":"desc"}}`
	rec := performRequest(wrapped, body)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestRefactorTipsHandlerStripsFences(t *testing.T) {
	provider := &mockProvider{
		generateContentFn: func(ctx context.Context, prompt, requestID, detailLevel string) (*models.GenerationResponse, error) {
			return &models.GenerationResponse{
				Content: "```text\nclean\n```",
				Metadata: models.GenerationMetadata{
					Model: "m",
				},
			}, nil
		},
	}
	handler := newTestAIHandler(provider, &mockPromptManager{})

	wrapped := middleware.ValidateRequest[*models.RefactorTipsRequest]()(http.HandlerFunc(handler.RefactorTipsHandler))
	body := `{"code":"print(1)","language":"python","question":{"prompt_markdown":"desc"}}`
	rec := performRequest(wrapped, body)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "clean") {
		t.Fatalf("expected refactor tips to be stripped: %s", rec.Body.String())
	}
}
