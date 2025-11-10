package jobs

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"peerprep/ai/internal/feedback"
	"peerprep/ai/internal/tuning"

	"github.com/robfig/cron/v3"
)

// FeedbackExporterJob handles automated export and fine-tuning
type FeedbackExporterJob struct {
	feedbackManager *feedback.FeedbackManager
	geminiTuner     *tuning.GeminiTuner
	config          *ExporterConfig
	cron            *cron.Cron
}

// ExporterConfig contains configuration for the exporter job
type ExporterConfig struct {
	Schedule          string // Cron schedule (e.g., "0 2 * * *" for 2 AM daily)
	ExportDir         string // Directory to store exported files
	ExportEnabled     bool   // Whether to run exports
	AutoTuneEnabled   bool   // Whether to automatically trigger fine-tuning
	MinSamplesForTune int    // Minimum positive feedback samples before tuning
	BaseModel         string // Base model for fine-tuning (e.g., "gemini-1.5-flash")
	LearningRate      float64
	EpochCount        int
	AdapterSize       string // "ADAPTER_SIZE_ONE", "ADAPTER_SIZE_FOUR", "ADAPTER_SIZE_EIGHT", "ADAPTER_SIZE_SIXTEEN"
}

// NewFeedbackExporterJob creates a new exporter job
func NewFeedbackExporterJob(
	feedbackManager *feedback.FeedbackManager,
	geminiTuner *tuning.GeminiTuner,
	config *ExporterConfig,
) *FeedbackExporterJob {
	return &FeedbackExporterJob{
		feedbackManager: feedbackManager,
		geminiTuner:     geminiTuner,
		config:          config,
		cron:            cron.New(),
	}
}

// Start begins the scheduled export job
func (fej *FeedbackExporterJob) Start() error {
	if !fej.config.ExportEnabled {
		log.Println("Feedback export is disabled, skipping scheduler")
		return nil
	}

	log.Printf("Starting feedback exporter with schedule: %s", fej.config.Schedule)

	// Schedule the job
	_, err := fej.cron.AddFunc(fej.config.Schedule, func() {
		if err := fej.RunExport(); err != nil {
			log.Printf("Export job failed: %v", err)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to schedule export job: %w", err)
	}

	fej.cron.Start()
	log.Println("Feedback exporter started successfully")

	return nil
}

// Stop stops the scheduled export job
func (fej *FeedbackExporterJob) Stop() {
	if fej.cron != nil {
		fej.cron.Stop()
		log.Println("Feedback exporter stopped")
	}
}

// RunExport performs a single export run
func (fej *FeedbackExporterJob) RunExport() error {
	log.Println("Starting feedback export job...")

	// Get unexported feedback
	feedback, err := fej.feedbackManager.GetUnexportedFeedback(0) // no limit
	if err != nil {
		return fmt.Errorf("failed to get unexported feedback: %w", err)
	}

	if len(feedback) == 0 {
		log.Println("No unexported feedback found")
		return nil
	}

	log.Printf("Found %d unexported feedback records", len(feedback))

	// Export to JSONL
	jsonlData, err := fej.feedbackManager.ExportToJSONL(feedback)
	if err != nil {
		return fmt.Errorf("failed to export to JSONL: %w", err)
	}

	// Count positive feedback (what actually gets exported)
	positiveCount := 0
	for _, fb := range feedback {
		if fb.IsPositive {
			positiveCount++
		}
	}

	if positiveCount == 0 {
		log.Println("No positive feedback to export, skipping file creation")
		// Still mark as exported to not process negative feedback again
		feedbackIDs := make([]uint, len(feedback))
		for i, fb := range feedback {
			feedbackIDs[i] = fb.ID
		}
		return fej.feedbackManager.MarkAsExported(feedbackIDs)
	}

	// Create export directory if it doesn't exist
	if err := os.MkdirAll(fej.config.ExportDir, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	// Save to file with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("feedback_export_%s.jsonl", timestamp)
	filepath := filepath.Join(fej.config.ExportDir, filename)

	if err := os.WriteFile(filepath, jsonlData, 0644); err != nil {
		return fmt.Errorf("failed to write export file: %w", err)
	}

	log.Printf("Exported %d positive feedback samples to %s", positiveCount, filepath)

	// Mark as exported
	feedbackIDs := make([]uint, len(feedback))
	for i, fb := range feedback {
		feedbackIDs[i] = fb.ID
	}

	if err := fej.feedbackManager.MarkAsExported(feedbackIDs); err != nil {
		return fmt.Errorf("failed to mark as exported: %w", err)
	}

	// Trigger auto-tuning if enabled and we have enough samples
	if fej.config.AutoTuneEnabled && positiveCount >= fej.config.MinSamplesForTune {
		log.Printf("Auto-tuning enabled and %d samples available (minimum: %d), starting fine-tuning job...",
			positiveCount, fej.config.MinSamplesForTune)

		if err := fej.RunFineTuning(filepath, positiveCount); err != nil {
			log.Printf("Fine-tuning job failed (export still succeeded): %v", err)
			// Don't return error - export was successful
		}
	} else if fej.config.AutoTuneEnabled {
		log.Printf("Auto-tuning enabled but only %d samples available (need %d), skipping fine-tuning",
			positiveCount, fej.config.MinSamplesForTune)
	}

	return nil
}

// RunFineTuning starts a fine-tuning job with the exported data
func (fej *FeedbackExporterJob) RunFineTuning(trainingFilePath string, sampleCount int) error {
	log.Println("Starting fine-tuning job...")

	ctx := context.Background()

	tuningConfig := &tuning.TuningConfig{
		BaseModel:        fej.config.BaseModel,
		TrainingFilePath: trainingFilePath, // NOTE: Must be GCS URI (gs://bucket/path/to/file.jsonl)
		LearningRate:     fej.config.LearningRate,
		EpochCount:       fej.config.EpochCount,
		AdapterSize:      fej.config.AdapterSize,
	}

	// Create tuning job
	modelVersion, err := fej.geminiTuner.CreateTuningJob(ctx, tuningConfig)
	if err != nil {
		return fmt.Errorf("failed to create tuning job: %w", err)
	}

	log.Printf("Fine-tuning job created: job_id=%s, version=%s", modelVersion.TrainingJobID, modelVersion.VersionName)

	// Wait for completion (with timeout)
	if err := fej.geminiTuner.WaitForCompletion(ctx, modelVersion.TrainingJobID, 24*time.Hour); err != nil {
		return fmt.Errorf("tuning job did not complete successfully: %w", err)
	}

	log.Printf("Fine-tuning job completed successfully: %s", modelVersion.VersionName)

	// Activate model with initial 10% traffic for A/B testing
	if err := fej.geminiTuner.ActivateModel(modelVersion.ID, 10); err != nil {
		return fmt.Errorf("failed to activate model: %w", err)
	}

	log.Printf("Model activated with 10%% traffic for A/B testing: %s", modelVersion.VersionName)

	return nil
}

// RunManual runs an export manually (for testing or on-demand exports)
func (fej *FeedbackExporterJob) RunManual() error {
	return fej.RunExport()
}
