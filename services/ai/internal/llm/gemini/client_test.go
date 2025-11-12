package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"peerprep/ai/internal/llm"
	"peerprep/ai/internal/models"

	"google.golang.org/genai"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newStubClient(t *testing.T, handler http.HandlerFunc) (*Client, func()) {
	t.Helper()
	server := httptest.NewServer(handler)

	genaiClient, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:     "test",
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: server.Client(),
		HTTPOptions: genai.HTTPOptions{
			BaseURL:    server.URL,
			APIVersion: "v1beta",
		},
	})
	if err != nil {
		t.Fatalf("failed to create genai client: %v", err)
	}

	client := &Client{
		apiKeyClient: genaiClient,
		vertexClient: nil,
		config:       &Config{APIKey: "test", Model: "test-model"},
	}

	return client, server.Close
}

func TestClientGenerateContentSuccess(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/test-model:generateContent" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		resp := map[string]any{
			"candidates": []map[string]any{
				{
					"content": map[string]any{
						"parts": []map[string]any{
							{"text": "hello world"},
						},
					},
				},
			},
			"modelVersion": "test-version",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}

	client, cleanup := newStubClient(t, handler)
	defer cleanup()

	resp, err := client.GenerateContent(context.Background(), "prompt", "req-1", "detail")
	if err != nil {
		t.Fatalf("GenerateContent returned error: %v", err)
	}
	if resp.Content != "hello world" {
		t.Fatalf("expected response text, got %s", resp.Content)
	}
	if resp.Metadata.Model != "test-model" {
		t.Fatalf("expected metadata to include model, got %+v", resp.Metadata)
	}
}

func TestClientGenerateContentRateLimit(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "429 rate limit", http.StatusTooManyRequests)
	}
	client, cleanup := newStubClient(t, handler)
	defer cleanup()

	_, err := client.GenerateContent(context.Background(), "prompt", "req", "detail")
	if err == nil {
		t.Fatal("expected error")
	}
	provErr, ok := err.(*llm.ProviderError)
	if !ok || provErr.Code != llm.ErrCodeRateLimit {
		t.Fatalf("expected provider rate limit error, got %v", err)
	}
}

func TestClientGenerateContentEmptyResponse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{"candidates": []map[string]any{{"content": map[string]any{"parts": []map[string]any{{"text": ""}}}}}}
		json.NewEncoder(w).Encode(resp)
	}
	client, cleanup := newStubClient(t, handler)
	defer cleanup()

	if _, err := client.GenerateContent(context.Background(), "prompt", "req", "detail"); err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestSelectModelWithDatabase(t *testing.T) {
	client := &Client{
		config: &Config{Model: "base"},
	}

	// no DB = base model
	model, version := client.selectModel(context.Background())
	if model != "base" || version != "" {
		t.Fatalf("expected base model, got %s version %s", model, version)
	}

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelVersion{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	if err := db.Create(&models.ModelVersion{
		VersionName:   "ft-model",
		IsActive:      true,
		TrafficWeight: 100,
	}).Error; err != nil {
		t.Fatalf("failed to seed model version: %v", err)
	}

	client.SetDatabase(db)
	model, version = client.selectModel(context.Background())
	if model != "ft-model" || version != "ft-model" {
		t.Fatalf("expected fine-tuned model selection, got %s / %s", model, version)
	}
}

func TestGetProviderNameAndRateLimitHelper(t *testing.T) {
	client := &Client{}
	if client.GetProviderName() != "gemini" {
		t.Fatalf("expected provider name gemini")
	}

	cases := map[string]bool{
		"429 rate limit exceeded": true,
		"RESOURCE_EXHAUSTED":      true,
		"quota exceeded":          true,
		"other error":             false,
	}
	for input, expect := range cases {
		if got := isRateLimitError(errors.New(input)); got != expect {
			t.Fatalf("isRateLimitError(%s) = %v, expected %v", input, got, expect)
		}
	}
	if isRateLimitError(nil) {
		t.Fatalf("expected nil error to return false")
	}
}
