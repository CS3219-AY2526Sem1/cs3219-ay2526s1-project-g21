package tuning

import (
	"fmt"
	"testing"
	"time"

	"peerprep/ai/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestTuner(t *testing.T) (*GeminiTuner, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.ModelVersion{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	tuner, err := NewGeminiTuner("", "project", db)
	if err != nil {
		t.Fatalf("failed to create tuner: %v", err)
	}
	return tuner, db
}

func seedModel(t *testing.T, db *gorm.DB, name string, active bool, weight int) *models.ModelVersion {
	t.Helper()
	model := &models.ModelVersion{
		VersionName:      name,
		BaseModel:        "base",
		TrainingJobID:    "job-" + name,
		TrainingDataSize: 1,
		IsActive:         active,
		TrafficWeight:    weight,
	}
	if err := db.Create(model).Error; err != nil {
		t.Fatalf("failed to seed model: %v", err)
	}
	return model
}

func TestActivateAndDeactivateModel(t *testing.T) {
	tuner, db := newTestTuner(t)
	model := seedModel(t, db, "model-a", false, 0)

	if err := tuner.ActivateModel(model.ID, 25); err != nil {
		t.Fatalf("ActivateModel error: %v", err)
	}

	var updated models.ModelVersion
	if err := db.First(&updated, model.ID).Error; err != nil {
		t.Fatalf("failed to fetch model: %v", err)
	}
	if !updated.IsActive || updated.TrafficWeight != 25 {
		t.Fatalf("expected active model with weight 25, got %+v", updated)
	}

	if err := tuner.DeactivateModel(model.ID); err != nil {
		t.Fatalf("DeactivateModel error: %v", err)
	}
	if err := db.First(&updated, model.ID).Error; err != nil {
		t.Fatalf("failed to fetch model: %v", err)
	}
	if updated.IsActive || updated.TrafficWeight != 0 {
		t.Fatalf("expected model to be inactive, got %+v", updated)
	}
}

func TestGetActiveModel(t *testing.T) {
	tuner, db := newTestTuner(t)
	seedModel(t, db, "model-a", true, 30)
	target := seedModel(t, db, "model-b", true, 60)

	active, err := tuner.GetActiveModel()
	if err != nil {
		t.Fatalf("GetActiveModel error: %v", err)
	}
	if active.ID != target.ID {
		t.Fatalf("expected model-b to be returned, got %+v", active)
	}
}
