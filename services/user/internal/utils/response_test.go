package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONWritesPayload(t *testing.T) {
	rec := httptest.NewRecorder()

	payload := map[string]any{"foo": "bar"}
	JSON(rec, http.StatusCreated, payload)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected content-type application/json, got %s", ct)
	}

	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if decoded["foo"] != "bar" {
		t.Fatalf("expected payload value 'bar', got %v", decoded["foo"])
	}
}

func TestJSONSkipsNilPayload(t *testing.T) {
	rec := httptest.NewRecorder()

	JSON(rec, http.StatusAccepted, nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestJSONErrorWrapsMessage(t *testing.T) {
	rec := httptest.NewRecorder()

	JSONError(rec, http.StatusBadRequest, "oops")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
	var decoded map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if decoded["error"] != "oops" {
		t.Fatalf("expected error message 'oops', got %q", decoded["error"])
	}
}
