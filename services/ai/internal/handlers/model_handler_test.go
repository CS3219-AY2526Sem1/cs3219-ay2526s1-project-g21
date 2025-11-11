package handlers

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"peerprep/ai/internal/models"
	"peerprep/ai/internal/tuning"

	"github.com/go-chi/chi/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type modelTestDeps struct {
	handler *ModelHandler
	db      *gorm.DB
}

func newModelTestDeps(t *testing.T) modelTestDeps {
	t.Helper()
	dsn := fmt.Sprintf("file:%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelVersion{}, &models.AIFeedback{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	tuner, err := tuning.NewGeminiTuner("", "project", db)
	if err != nil {
		t.Fatalf("failed to create tuner: %v", err)
	}

	return modelTestDeps{
		handler: NewModelHandler(db, tuner),
		db:      db,
	}
}

func seedModelVersion(t *testing.T, db *gorm.DB, weight int) *models.ModelVersion {
	t.Helper()
	model := &models.ModelVersion{
		VersionName:      fmt.Sprintf("model-%d", time.Now().UnixNano()),
		BaseModel:        "base",
		TrainingJobID:    fmt.Sprintf("job-%d", time.Now().UnixNano()),
		TrainingDataSize: 10,
		IsActive:         true,
		TrafficWeight:    weight,
	}
	if err := db.Create(model).Error; err != nil {
		t.Fatalf("failed to seed model: %v", err)
	}
	return model
}

func performModelRequest(handler http.HandlerFunc, method, modelID string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/", bytes.NewBuffer(body))
	if modelID != "" {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("model_id", modelID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestUpdateTrafficWeight(t *testing.T) {
	deps := newModelTestDeps(t)
	model := seedModelVersion(t, deps.db, 0)

	rec := performModelRequest(deps.handler.UpdateTrafficWeight, http.MethodPut, "", []byte(`{"traffic_weight":30}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing model_id path param should result in 400, got %d", rec.Code)
	}

	rec = performModelRequest(deps.handler.UpdateTrafficWeight, http.MethodPut, strconv.Itoa(int(model.ID)), []byte(`{"traffic_weight":30}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var updated models.ModelVersion
	if err := deps.db.First(&updated, model.ID).Error; err != nil {
		t.Fatalf("failed to fetch updated model: %v", err)
	}
	if updated.TrafficWeight != 30 {
		t.Fatalf("expected weight updated to 30, got %d", updated.TrafficWeight)
	}
}

func TestUpdateTrafficWeightValidation(t *testing.T) {
	deps := newModelTestDeps(t)
	seedModelVersion(t, deps.db, 0)

	rec := performModelRequest(deps.handler.UpdateTrafficWeight, http.MethodPut, "not-number", []byte(`{"traffic_weight":30}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", rec.Code)
	}

	rec = performModelRequest(deps.handler.UpdateTrafficWeight, http.MethodPut, "1", []byte(`{`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", rec.Code)
	}

	rec = performModelRequest(deps.handler.UpdateTrafficWeight, http.MethodPut, "1", []byte(`{"traffic_weight":101}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid weight, got %d", rec.Code)
	}
}

func TestDeactivateModel(t *testing.T) {
	deps := newModelTestDeps(t)
	model := seedModelVersion(t, deps.db, 50)

	rec := performModelRequest(deps.handler.DeactivateModel, http.MethodPut, "not-id", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id")
	}

	rec = performModelRequest(deps.handler.DeactivateModel, http.MethodPut, strconv.Itoa(int(model.ID)), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var updated models.ModelVersion
	if err := deps.db.First(&updated, model.ID).Error; err != nil {
		t.Fatalf("failed to fetch updated model: %v", err)
	}
	if updated.IsActive || updated.TrafficWeight != 0 {
		t.Fatalf("expected model to be deactivated, got %+v", updated)
	}
}

func TestListModels(t *testing.T) {
	deps := newModelTestDeps(t)
	seedModelVersion(t, deps.db, 10)
	seedModelVersion(t, deps.db, 20)

	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	rec := httptest.NewRecorder()
	deps.handler.ListModels(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/models?active=true", nil)
	rec = httptest.NewRecorder()
	deps.handler.ListModels(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestGetModelStats(t *testing.T) {
	deps := newModelTestDeps(t)
	model := seedModelVersion(t, deps.db, 10)
	deps.db.Create(&models.AIFeedback{
		RequestID:    "req",
		RequestType:  "hint",
		Prompt:       "p",
		Response:     "r",
		IsPositive:   true,
		ModelVersion: model.VersionName,
		FeedbackAt:   time.Now(),
	})

	rec := performModelRequest(deps.handler.GetModelStats, http.MethodGet, "not-id", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id")
	}

	rec = performModelRequest(deps.handler.GetModelStats, http.MethodGet, "999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing model, got %d", rec.Code)
	}

	rec = performModelRequest(deps.handler.GetModelStats, http.MethodGet, strconv.Itoa(int(model.ID)), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"positive_rate"`)) {
		t.Fatalf("expected stats in response, got %s", rec.Body.String())
	}
}
