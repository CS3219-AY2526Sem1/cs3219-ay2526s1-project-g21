package feedback

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"peerprep/ai/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestFeedbackManager(t *testing.T) *FeedbackManager {
	t.Helper()

	dsn := fmt.Sprintf("file:%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&models.AIFeedback{}, &models.ModelVersion{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return NewFeedbackManager(db, time.Minute)
}

func TestSubmitFeedbackSuccess(t *testing.T) {
	fm := newTestFeedbackManager(t)

	ctx := &models.RequestContext{
		RequestID:    "req-1",
		RequestType:  "explain",
		Prompt:       "prompt",
		Response:     "response",
		ModelVersion: "v1",
	}
	fm.StoreRequestContext(ctx)

	if err := fm.SubmitFeedback("req-1", true); err != nil {
		t.Fatalf("SubmitFeedback returned error: %v", err)
	}

	var stored models.AIFeedback
	if err := fm.db.First(&stored, "request_id = ?", "req-1").Error; err != nil {
		t.Fatalf("expected stored feedback, got error: %v", err)
	}
	if !stored.IsPositive {
		t.Fatalf("expected positive feedback to be stored")
	}

	if fm.contextCache.Size() != 0 {
		t.Fatalf("expected context cache to be cleared after storing feedback")
	}
}

func TestSubmitFeedbackMissingContext(t *testing.T) {
	fm := newTestFeedbackManager(t)
	if err := fm.SubmitFeedback("missing", false); err == nil {
		t.Fatal("expected error when context missing")
	}
}

func seedFeedback(t *testing.T, fm *FeedbackManager, exported bool, positive bool, ts time.Time) models.AIFeedback {
	t.Helper()
	fb := models.AIFeedback{
		RequestID:    ts.Format(time.RFC3339Nano),
		RequestType:  "hint",
		Prompt:       "prompt",
		Response:     "response",
		IsPositive:   positive,
		ModelVersion: "v1",
		FeedbackAt:   ts,
		Exported:     exported,
	}
	if err := fm.db.Create(&fb).Error; err != nil {
		t.Fatalf("failed seeding feedback: %v", err)
	}
	return fb
}

func TestGetUnexportedFeedback(t *testing.T) {
	fm := newTestFeedbackManager(t)
	older := seedFeedback(t, fm, false, true, time.Now().Add(-time.Hour))
	seedFeedback(t, fm, false, false, time.Now())
	seedFeedback(t, fm, true, true, time.Now())

	results, err := fm.GetUnexportedFeedback(1)
	if err != nil {
		t.Fatalf("GetUnexportedFeedback error: %v", err)
	}
	if len(results) != 1 || results[0].ID != older.ID {
		t.Fatalf("expected oldest unexported feedback first, got %+v", results)
	}
}

func TestGetFeedbackSince(t *testing.T) {
	fm := newTestFeedbackManager(t)
	seedFeedback(t, fm, false, true, time.Now().Add(-48*time.Hour))
	target := seedFeedback(t, fm, false, true, time.Now())

	results, err := fm.GetFeedbackSince(time.Now().Add(-24*time.Hour), 10)
	if err != nil {
		t.Fatalf("GetFeedbackSince error: %v", err)
	}
	if len(results) != 1 || results[0].ID != target.ID {
		t.Fatalf("expected only recent feedback, got %+v", results)
	}
}

func TestMarkAsExported(t *testing.T) {
	fm := newTestFeedbackManager(t)
	fb := seedFeedback(t, fm, false, true, time.Now())

	if err := fm.MarkAsExported([]uint{fb.ID}); err != nil {
		t.Fatalf("MarkAsExported error: %v", err)
	}

	var updated models.AIFeedback
	if err := fm.db.First(&updated, fb.ID).Error; err != nil {
		t.Fatalf("failed to fetch feedback: %v", err)
	}
	if !updated.Exported || updated.ExportedAt == nil {
		t.Fatalf("expected feedback to be marked exported with timestamp, got %+v", updated)
	}
}

func TestExportToJSONL(t *testing.T) {
	fm := newTestFeedbackManager(t)

	feedback := []models.AIFeedback{
		{Prompt: "prompt1", Response: "resp1", IsPositive: true},
		{Prompt: "prompt2", Response: "resp2", IsPositive: false},
	}

	data, err := fm.ExportToJSONL(feedback)
	if err != nil {
		t.Fatalf("ExportToJSONL error: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected only positive feedback exported, got %d lines", len(lines))
	}

	var parsed models.TrainingDataPoint
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("failed to unmarshal exported line: %v", err)
	}
	if parsed.Contents[0].Parts[0].Text != "prompt1" {
		t.Fatalf("unexpected prompt in export: %+v", parsed)
	}
}

func TestGetFeedbackStats(t *testing.T) {
	fm := newTestFeedbackManager(t)
	seedFeedback(t, fm, false, true, time.Now())
	seedFeedback(t, fm, false, false, time.Now())

	stats, err := fm.GetFeedbackStats()
	if err != nil {
		t.Fatalf("GetFeedbackStats error: %v", err)
	}

	if stats["total_count"].(int64) != 2 {
		t.Fatalf("expected total_count 2, got %+v", stats["total_count"])
	}
	if stats["positive_count"].(int64) != 1 {
		t.Fatalf("expected positive_count 1, got %+v", stats["positive_count"])
	}
	if stats["unexported_count"].(int64) != 2 {
		t.Fatalf("expected unexported_count 2, got %+v", stats["unexported_count"])
	}

	fm.StoreRequestContext(&models.RequestContext{RequestID: "cache"})
	if stats, err = fm.GetFeedbackStats(); err != nil {
		t.Fatalf("GetFeedbackStats error: %v", err)
	}
	if stats["cached_contexts"].(int) != 1 {
		t.Fatalf("expected cached_contexts 1, got %+v", stats["cached_contexts"])
	}
}
