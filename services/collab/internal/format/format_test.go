package format

import (
	"context"
	"testing"

	"collab/internal/models"
)

func TestFormatReturnsSource(t *testing.T) {
	out, err := Format(context.Background(), models.FormatRequest{
		Language: models.LangPython,
		Code:     "print('hi')",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "print('hi')" {
		t.Fatalf("expected original code, got %q", out)
	}
}

func TestFormatContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Format(ctx, models.FormatRequest{Language: models.LangPython, Code: "x"}); err == nil {
		t.Fatalf("expected context cancellation error")
	}
}
