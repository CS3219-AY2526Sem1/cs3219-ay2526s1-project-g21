package llm

import (
	"context"
	"errors"
	"testing"

	"peerprep/ai/internal/models"
)

type testProvider struct{}

func (testProvider) GenerateContent(context.Context, string, string, string) (*models.GenerationResponse, error) {
	return &models.GenerationResponse{Content: "ok"}, nil
}
func (testProvider) GetProviderName() string { return "test" }

func TestProviderErrorError(t *testing.T) {
	err := &ProviderError{Provider: "gemini", Message: "failed"}
	if err.Error() != "gemini error: failed" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}

	wrapped := &ProviderError{Provider: "gemini", Message: "failed", Err: errors.New("detail")}
	if got := wrapped.Error(); got != "gemini error: failed (detail)" {
		t.Fatalf("unexpected wrapped error message: %s", got)
	}
}

func TestRegisterAndNewProvider(t *testing.T) {
	RegisterProvider("test_provider", func() (Provider, error) {
		return testProvider{}, nil
	})
	defer delete(providers, "test_provider")

	provider, err := NewProvider("test_provider")
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}
	if name := provider.GetProviderName(); name != "test" {
		t.Fatalf("expected provider name test, got %s", name)
	}

	if _, err := NewProvider("missing"); err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}
