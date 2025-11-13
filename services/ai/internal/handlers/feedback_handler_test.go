package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"peerprep/ai/internal/feedback"
	"peerprep/ai/internal/models"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newFeedbackHandlerWithDB(t *testing.T) (*FeedbackHandler, *feedback.FeedbackManager) {
	t.Helper()
	dsn := fmt.Sprintf("file:%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AIFeedback{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	manager := feedback.NewFeedbackManager(db, time.Minute)
	return NewFeedbackHandler(manager), manager
}

func addURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestSubmitFeedbackFlow(t *testing.T) {
	handler, manager := newFeedbackHandlerWithDB(t)
	manager.StoreRequestContext(&models.RequestContext{
		RequestID:    "req-1",
		RequestType:  "explain",
		Prompt:       "prompt",
		Response:     "response",
		ModelVersion: "v1",
	})

	req := httptest.NewRequest(http.MethodPost, "/feedback/req-1", bytes.NewBufferString(`{"is_positive":true}`))
	req = addURLParam(req, "request_id", "req-1")
	rec := httptest.NewRecorder()

	handler.SubmitFeedback(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestSubmitFeedbackValidationErrors(t *testing.T) {
	handler, _ := newFeedbackHandlerWithDB(t)

	req := httptest.NewRequest(http.MethodPost, "/feedback/", nil)
	rec := httptest.NewRecorder()
	handler.SubmitFeedback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing request id, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/feedback/req-1", bytes.NewBufferString(`{`))
	req = addURLParam(req, "request_id", "req-1")
	rec = httptest.NewRecorder()
	handler.SubmitFeedback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/feedback/req-2", bytes.NewBufferString(`{"is_positive":false}`))
	req = addURLParam(req, "request_id", "req-2")
	rec = httptest.NewRecorder()
	handler.SubmitFeedback(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when context missing, got %d", rec.Code)
	}
}

func insertFeedback(t *testing.T, manager *feedback.FeedbackManager, positive bool) {
	t.Helper()
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	manager.StoreRequestContext(&models.RequestContext{
		RequestID:    requestID,
		RequestType:  "hint",
		Prompt:       "p",
		Response:     "r",
		ModelVersion: "v1",
	})
	if err := manager.SubmitFeedback(requestID, positive); err != nil {
		t.Fatalf("failed to store feedback: %v", err)
	}
}

func TestExportFeedbackFlows(t *testing.T) {
	handler, manager := newFeedbackHandlerWithDB(t)
	insertFeedback(t, manager, true)
	insertFeedback(t, manager, false)

	req := httptest.NewRequest(http.MethodGet, "/feedback/export?format=jsonl", nil)
	rec := httptest.NewRecorder()
	handler.ExportFeedback(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/feedback/export?format=json", nil)
	rec = httptest.NewRecorder()
	handler.ExportFeedback(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for json export, got %d", rec.Code)
	}

	handler, _ = newFeedbackHandlerWithDB(t)
	req = httptest.NewRequest(http.MethodGet, "/feedback/export", nil)
	rec = httptest.NewRecorder()
	handler.ExportFeedback(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with message for empty data, got %d", rec.Code)
	}
}

func TestGetFeedbackStats(t *testing.T) {
	handler, manager := newFeedbackHandlerWithDB(t)
	insertFeedback(t, manager, true)

	req := httptest.NewRequest(http.MethodGet, "/feedback/stats", nil)
	rec := httptest.NewRecorder()
	handler.GetFeedbackStats(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// NOTE: missing data still returns OK
	// DB issues NOT simulated here
}
