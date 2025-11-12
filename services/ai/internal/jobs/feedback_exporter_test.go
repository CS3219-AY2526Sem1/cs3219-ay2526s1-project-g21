package jobs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"peerprep/ai/internal/feedback"
	"peerprep/ai/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newFeedbackManager(t *testing.T) *feedback.FeedbackManager {
	t.Helper()
	dsn := fmt.Sprintf("file:%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AIFeedback{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return feedback.NewFeedbackManager(db, time.Minute)
}

func storeFeedbackSample(t *testing.T, manager *feedback.FeedbackManager, positive bool) {
	t.Helper()
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
	manager.StoreRequestContext(&models.RequestContext{
		RequestID:    requestID,
		RequestType:  "hint",
		Prompt:       "prompt",
		Response:     "response",
		ModelVersion: "v1",
	})
	if err := manager.SubmitFeedback(requestID, positive); err != nil {
		t.Fatalf("failed to submit feedback: %v", err)
	}
}

func TestRunExport_NoData(t *testing.T) {
	manager := newFeedbackManager(t)
	exportDir := t.TempDir()

	job := NewFeedbackExporterJob(manager, nil, &ExporterConfig{
		ExportDir:     exportDir,
		ExportEnabled: true,
	})

	if err := job.RunExport(); err != nil {
		t.Fatalf("RunExport with no data should not error, got %v", err)
	}
}

func TestRunExport_WithPositiveData(t *testing.T) {
	manager := newFeedbackManager(t)
	storeFeedbackSample(t, manager, true)
	storeFeedbackSample(t, manager, false)

	exportDir := t.TempDir()
	job := NewFeedbackExporterJob(manager, nil, &ExporterConfig{
		ExportDir:       exportDir,
		ExportEnabled:   true,
		AutoTuneEnabled: false,
	})

	if err := job.RunExport(); err != nil {
		t.Fatalf("RunExport returned error: %v", err)
	}

	files, err := os.ReadDir(exportDir)
	if err != nil {
		t.Fatalf("failed to read export dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one export file, got %d", len(files))
	}

	// export -> feedback marked as exported
	records, err := manager.GetUnexportedFeedback(10)
	if err != nil {
		t.Fatalf("failed to fetch feedback: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected all feedback to be marked exported, got %d", len(records))
	}

	content, err := os.ReadFile(filepath.Join(exportDir, files[0].Name()))
	if err != nil {
		t.Fatalf("failed to read export file: %v", err)
	}
	if len(content) == 0 {
		t.Fatalf("expected export file to contain data")
	}
}

func TestExporterStartStop(t *testing.T) {
	manager := newFeedbackManager(t)
	job := NewFeedbackExporterJob(manager, nil, &ExporterConfig{
		ExportEnabled: false,
	})

	if err := job.Start(); err != nil {
		t.Fatalf("disabled exporter should not error, got %v", err)
	}

	job.config.ExportEnabled = true
	job.config.Schedule = "@every 1m"
	if err := job.Start(); err != nil {
		t.Fatalf("expected scheduler to start, got %v", err)
	}
	job.Stop()
}
