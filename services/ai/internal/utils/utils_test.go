package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeHelpers(t *testing.T) {
	t.Helper()

	if got := NormalizeLanguage("  Python "); got != "python" {
		t.Fatalf("NormalizeLanguage: expected python, got %s", got)
	}

	if got := NormalizeLevel(" Beginner"); got != "beginner" {
		t.Fatalf("NormalizeLevel: expected beginner, got %s", got)
	}

	if got := NormalizeDifficulty("  Medium "); got != "medium" {
		t.Fatalf("NormalizeDifficulty: expected medium, got %s", got)
	}
}

func TestStripFences(t *testing.T) {
	input := "```python\nprint('hi')\n```\n"
	want := "print('hi')"

	if got := StripFences(input); got != want {
		t.Fatalf("StripFences: expected %q, got %q", want, got)
	}

	raw := "  print('hi')  "
	if got := StripFences(raw); got != "print('hi')" {
		t.Fatalf("StripFences (no fences): expected trimmed string, got %q", got)
	}
}

func TestAddLineNumbers(t *testing.T) {
	code := "line1\nline2"
	want := "1: line1\n2: line2"

	if got := AddLineNumbers(code); got != want {
		t.Fatalf("AddLineNumbers: expected %q, got %q", want, got)
	}

	if got := AddLineNumbers(""); got != "" {
		t.Fatalf("AddLineNumbers empty: expected empty string, got %q", got)
	}
}

func TestJSONHelpers(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"hello": "world"}

	JSON(rec, http.StatusCreated, payload)

	if rec.Code != http.StatusCreated {
		t.Fatalf("JSON: expected status %d, got %d", http.StatusCreated, rec.Code)
	}
	if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("JSON: expected content-type application/json, got %s", contentType)
	}

	var got map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}
	if got["hello"] != "world" {
		t.Fatalf("JSON body mismatch: %+v", got)
	}

	rec2 := httptest.NewRecorder()
	WriteJSON(rec2, http.StatusAccepted, payload)

	if rec2.Code != http.StatusAccepted {
		t.Fatalf("WriteJSON: expected status %d, got %d", http.StatusAccepted, rec2.Code)
	}

	if !strings.Contains(rec2.Body.String(), `"hello":"world"`) {
		t.Fatalf("WriteJSON: expected body to contain payload, got %s", rec2.Body.String())
	}
}
