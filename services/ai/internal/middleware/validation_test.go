package middleware

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"peerprep/ai/internal/models"
)

type mockRequest struct {
	Value string `json:"value"`
}

func (m *mockRequest) Validate() error {
	switch m.Value {
	case "error_response":
		return &models.ErrorResponse{Code: "invalid", Message: "invalid"}
	case "generic_error":
		return errors.New("failed")
	default:
		return nil
	}
}

func TestValidateRequestSuccess(t *testing.T) {
	middlewareFn := ValidateRequest[*mockRequest]()
	called := false

	handler := middlewareFn(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		req := GetValidatedRequest[*mockRequest](r)
		if req.Value != "ok" {
			t.Fatalf("expected value ok, got %s", req.Value)
		}
	}))

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"value":"ok"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestValidateRequestInvalidJSON(t *testing.T) {
	middlewareFn := ValidateRequest[*mockRequest]()
	handler := middlewareFn(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json, got %d", rec.Code)
	}
}

func TestValidateRequestValidationErrors(t *testing.T) {
	t.Run("error response", func(t *testing.T) {
		middlewareFn := ValidateRequest[*mockRequest]()
		handler := middlewareFn(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"value":"error_response"}`))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for validation error, got %d", rec.Code)
		}
	})

	t.Run("generic error", func(t *testing.T) {
		middlewareFn := ValidateRequest[*mockRequest]()
		handler := middlewareFn(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(`{"value":"generic_error"}`))
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for validation error, got %d", rec.Code)
		}
	})
}
