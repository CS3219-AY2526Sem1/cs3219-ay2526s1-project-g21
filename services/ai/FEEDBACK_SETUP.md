# AI Feedback & Fine-Tuning System

This document explains the automated LLM feedback collection and fine-tuning system integrated with the Gemini API.

## Overview

The system allows users to provide thumbs up/down feedback on AI-generated responses (explanations, hints, test cases, refactor tips). This feedback is:
1. Stored in PostgreSQL (without user IDs for privacy)
2. Automatically exported to JSONL format on a schedule
3. Used to fine-tune the Gemini model via Google's Fine-Tuning API
4. Deployed with A/B testing for gradual rollout

## Architecture

```
User provides feedback → Store in PostgreSQL (ai_feedback table)
    ↓ (Daily cron at 2 AM)
Query last 7 days feedback → Generate JSONL → Upload to Gemini
    ↓ (If ENABLE_AUTO_TUNING=true)
Call Gemini Fine-Tuning API → Wait for completion
    ↓
Store new model version → A/B test (10% traffic)
    ↓ (If metrics improve)
Gradual rollout → Full production
```

## Components

### Backend (services/ai/)

#### 1. Database Models ([internal/models/feedback.go](internal/models/feedback.go))

**AIFeedback** - Stores feedback without user IDs:
```go
type AIFeedback struct {
    RequestID    string    // Unique request identifier
    RequestType  string    // "hint", "explain", "tests", "refactor_tips"
    Prompt       string    // Input prompt
    Response     string    // AI response
    IsPositive   bool      // true = thumbs up, false = thumbs down
    ModelVersion string    // e.g., "gemini-1.5-flash"
    FeedbackAt   time.Time
    Exported     bool      // Tracking export status
    ExportedAt   *time.Time
}
```

**ModelVersion** - Tracks fine-tuned models:
```go
type ModelVersion struct {
    VersionName      string  // e.g., "gemini-1.5-flash-ft-001"
    BaseModel        string
    TrainingJobID    string
    TrainingDataSize int
    IsActive         bool
    TrafficWeight    int  // 0-100 percentage for A/B testing
}
```

**RequestContext** - In-memory cache (15min TTL):
```go
type RequestContext struct {
    RequestID    string
    RequestType  string
    Prompt       string
    Response     string
    ModelVersion string
    Timestamp    time.Time
}
```

#### 2. Feedback Manager ([internal/feedback/feedback_manager.go](internal/feedback/feedback_manager.go))

Handles feedback storage and export:
- `StoreRequestContext()` - Cache request/response pairs
- `SubmitFeedback()` - Store user feedback
- `GetUnexportedFeedback()` - Query unexported feedback
- `ExportToJSONL()` - Convert to Gemini training format
- `MarkAsExported()` - Track exported records

#### 3. Context Cache ([internal/feedback/context_cache.go](internal/feedback/context_cache.go))

In-memory cache with TTL:
- 15-minute expiration (configurable)
- Background cleanup every 5 minutes
- Thread-safe with RWMutex

#### 4. Gemini Tuner ([internal/tuning/gemini_tuner.go](internal/tuning/gemini_tuner.go))

Integrates with Gemini Fine-Tuning API:
- `CreateTuningJob()` - Upload training data and start tuning
- `GetTuningJobStatus()` - Monitor job progress
- `WaitForCompletion()` - Block until tuning completes
- `ActivateModel()` - Enable model with traffic weight
- `DeactivateModel()` - Disable model

#### 5. Feedback Exporter Job ([internal/jobs/feedback_exporter.go](internal/jobs/feedback_exporter.go))

Automated cron job:
- Runs daily at 2 AM (configurable)
- Exports unexported feedback to JSONL
- Optionally triggers fine-tuning if enough samples
- Activates new model with 10% traffic for A/B testing

#### 6. Handlers ([internal/handlers/](internal/handlers/))

**feedback_handler.go**:
- `POST /api/v1/ai/feedback/:request_id` - Submit feedback
- `GET /api/v1/ai/feedback/export` - Export JSONL (manual)
- `GET /api/v1/ai/feedback/stats` - Get feedback statistics

**ai_handler.go** (modified):
- Stores request context after each AI response
- Tracks request IDs for feedback linking

### Frontend (peerprep-frontend/)

#### 1. API Client ([src/api/ai.ts](../../../peerprep-frontend/src/api/ai.ts))

```typescript
export async function submitAIFeedback(requestId: string, isPositive: boolean): Promise<void>
```

#### 2. Hook ([src/hooks/useAi.ts](../../../peerprep-frontend/src/hooks/useAi.ts))

Tracks request IDs:
```typescript
const { run, loading, text, error, requestId, setText, setError } = useExplain();
```

#### 3. AI Assistant Component ([src/components/AiAssistant.tsx](../../../peerprep-frontend/src/components/AiAssistant.tsx))

Features:
- Thumbs up/down buttons below AI responses
- Visual feedback on click (green/red highlight)
- Disabled after feedback given
- Tracks request IDs from all modes (Explain, Hint, Tests, Refactor)

## Configuration

Add to `.env`:

```env
# Gemini API
GEMINI_API_KEY=your_api_key_here
GEMINI_MODEL=gemini-1.5-flash
GEMINI_PROJECT_ID=your-gcp-project-id

# Feedback & Fine-Tuning
FEEDBACK_CACHE_TTL=15m
FEEDBACK_EXPORT_ENABLED=true
FEEDBACK_EXPORT_SCHEDULE=0 2 * * *          # Daily at 2 AM
FEEDBACK_EXPORT_DIR=./exports
FEEDBACK_AUTO_TUNE_ENABLED=false            # Set true when ready
FEEDBACK_MIN_SAMPLES_FOR_TUNE=100           # Minimum positive feedback
TUNING_BASE_MODEL=gemini-1.5-flash
TUNING_LEARNING_RATE=0.001
TUNING_EPOCH_COUNT=5
TUNING_BATCH_SIZE=4
```

## Database Setup

Run migrations to create tables:

```sql
-- ai_feedback table (automatically created by GORM AutoMigrate)
CREATE TABLE ai_feedback (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    request_id VARCHAR UNIQUE NOT NULL,
    request_type VARCHAR NOT NULL,
    prompt TEXT NOT NULL,
    response TEXT NOT NULL,
    is_positive BOOLEAN NOT NULL,
    model_version VARCHAR NOT NULL,
    feedback_at TIMESTAMP NOT NULL,
    exported BOOLEAN DEFAULT FALSE,
    exported_at TIMESTAMP
);

CREATE INDEX idx_ai_feedback_exported ON ai_feedback(exported);
CREATE INDEX idx_ai_feedback_deleted_at ON ai_feedback(deleted_at);

-- model_version table
CREATE TABLE model_versions (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP,
    version_name VARCHAR UNIQUE NOT NULL,
    base_model VARCHAR NOT NULL,
    training_job_id VARCHAR UNIQUE,
    training_data_size INTEGER NOT NULL,
    is_active BOOLEAN DEFAULT FALSE,
    traffic_weight INTEGER DEFAULT 0,
    activated_at TIMESTAMP,
    deactivated_at TIMESTAMP
);
```

## Integration with Main Service

Update `services/ai/cmd/server/main.go`:

```go
import (
    "peerprep/ai/internal/feedback"
    "peerprep/ai/internal/tuning"
    "peerprep/ai/internal/jobs"
    "time"
)

func main() {
    // ... existing setup ...

    // Initialize feedback manager
    cacheTTL, _ := time.ParseDuration(os.Getenv("FEEDBACK_CACHE_TTL"))
    if cacheTTL == 0 {
        cacheTTL = 15 * time.Minute
    }
    feedbackManager := feedback.NewFeedbackManager(db, cacheTTL)

    // Set feedback manager on AI handler
    aiHandler.SetFeedbackManager(feedbackManager)

    // Initialize Gemini tuner (if auto-tuning enabled)
    autoTuneEnabled := os.Getenv("FEEDBACK_AUTO_TUNE_ENABLED") == "true"
    var geminiTuner *tuning.GeminiTuner
    if autoTuneEnabled {
        geminiTuner, err = tuning.NewGeminiTuner(
            os.Getenv("GEMINI_API_KEY"),
            os.Getenv("GEMINI_PROJECT_ID"),
            db,
        )
        if err != nil {
            log.Fatal(err)
        }
    }

    // Initialize and start exporter job
    exporterConfig := &jobs.ExporterConfig{
        Schedule:          os.Getenv("FEEDBACK_EXPORT_SCHEDULE"),
        ExportDir:         os.Getenv("FEEDBACK_EXPORT_DIR"),
        ExportEnabled:     os.Getenv("FEEDBACK_EXPORT_ENABLED") == "true",
        AutoTuneEnabled:   autoTuneEnabled,
        MinSamplesForTune: getEnvInt("FEEDBACK_MIN_SAMPLES_FOR_TUNE", 100),
        BaseModel:         os.Getenv("TUNING_BASE_MODEL"),
        LearningRate:      getEnvFloat("TUNING_LEARNING_RATE", 0.001),
        EpochCount:        getEnvInt("TUNING_EPOCH_COUNT", 5),
        BatchSize:         getEnvInt("TUNING_BATCH_SIZE", 4),
    }

    exporterJob := jobs.NewFeedbackExporterJob(feedbackManager, geminiTuner, exporterConfig)
    if err := exporterJob.Start(); err != nil {
        log.Printf("Failed to start exporter job: %v", err)
    }
    defer exporterJob.Stop()

    // Initialize feedback handler
    feedbackHandler := handlers.NewFeedbackHandler(feedbackManager)

    // Update routes
    routers.AIRoutes(router, aiHandler, feedbackHandler)

    // ... rest of server setup ...
}
```

## Usage Flow

### 1. User Requests AI Response

```
User clicks "Explain Code"
  → POST /api/v1/ai/explain
  → AI generates response
  → Handler stores RequestContext in cache (15min TTL)
  → Response returned with request_id
```

### 2. User Provides Feedback

```
User clicks thumbs up/down
  → POST /api/v1/ai/feedback/{request_id} with {is_positive: true/false}
  → FeedbackHandler retrieves context from cache
  → Stores AIFeedback in PostgreSQL
  → Removes context from cache
```

### 3. Automated Export (Daily at 2 AM)

```
Cron job triggers
  → Query unexported feedback
  → Convert to JSONL format:
     {
       "contents": [
         {"role": "user", "parts": [{"text": "prompt"}]},
         {"role": "model", "parts": [{"text": "response"}]}
       ]
     }
  → Save to ./exports/feedback_export_YYYYMMDD_HHMMSS.jsonl
  → Mark records as exported
```

### 4. Automated Fine-Tuning (if enabled & enough samples)

```
If positive_count >= MIN_SAMPLES_FOR_TUNE:
  → Upload JSONL to Gemini Files API
  → Create tuned model via Gemini API
  → Wait for completion (up to 24 hours)
  → Store ModelVersion in database
  → Activate with 10% traffic weight
```

### 5. A/B Testing & Rollout

```
10% of requests use fine-tuned model
  → Monitor performance metrics
  → If improved: increase traffic weight (50% → 100%)
  → If not improved: deactivate model
```

## JSONL Export Format

Only positive feedback (thumbs up) is exported for training:

```jsonl
{"contents":[{"role":"user","parts":[{"text":"Explain this Python code: def fib(n)..."}]},{"role":"model","parts":[{"text":"This is a recursive Fibonacci function..."}]}]}
{"contents":[{"role":"user","parts":[{"text":"Give me a hint for..."}]},{"role":"model","parts":[{"text":"Consider using dynamic programming..."}]}]}
```

## Manual Export

For manual exports (e.g., during testing):

```bash
# Export last 7 days of feedback as JSONL
curl "http://localhost:8086/api/v1/ai/feedback/export?days=7&format=jsonl" -o export.jsonl

# Export as JSON for inspection
curl "http://localhost:8086/api/v1/ai/feedback/export?days=7&format=json"

# Get statistics
curl "http://localhost:8086/api/v1/ai/feedback/stats"
```

Response:
```json
{
  "ok": true,
  "info": {
    "total_count": 245,
    "positive_count": 198,
    "unexported_count": 42,
    "cached_contexts": 8
  }
}
```

## Monitoring

### Check Active Model

```go
activeModel, err := geminiTuner.GetActiveModel()
if activeModel != nil {
    log.Printf("Active model: %s with %d%% traffic",
        activeModel.VersionName, activeModel.TrafficWeight)
}
```

### View Feedback Stats

```bash
curl http://localhost:8086/api/v1/ai/feedback/stats
```

### Check Export Logs

```bash
tail -f logs/ai-service.log | grep "feedback_export"
```

## Privacy & Security

1. **No User IDs**: Feedback records exclude user identifiers
2. **Cache TTL**: Request contexts expire after 15 minutes
3. **Lazy Storage**: Only stores feedback when user explicitly provides it
4. **Anonymous Training**: Fine-tuning data contains no PII

## Troubleshooting

### Feedback not being stored

- Check cache TTL (request contexts expire after 15 minutes)
- Verify request_id from AI response matches feedback submission
- Check logs for cache hits/misses

### Export job not running

- Verify `FEEDBACK_EXPORT_ENABLED=true`
- Check cron schedule syntax: `0 2 * * *`
- Ensure export directory exists and is writable

### Fine-tuning not triggering

- Set `FEEDBACK_AUTO_TUNE_ENABLED=true`
- Verify positive feedback count >= `MIN_SAMPLES_FOR_TUNE`
- Check Gemini API key and project ID
- Review tuning job logs for errors

### Model not activating

- Check Gemini API quotas and limits
- Verify tuning job completed successfully
- Ensure database connection for ModelVersion storage

## Future Enhancements

1. **Feedback Comments**: Add optional text feedback (currently thumbs only)
2. **User Segmentation**: Track feedback by user type (without storing user IDs)
3. **Multi-Model Testing**: A/B test multiple fine-tuned models simultaneously
4. **Metrics Dashboard**: Visualize feedback trends and model performance
5. **Automatic Rollback**: Revert to base model if metrics degrade
6. **Incremental Training**: Combine new feedback with previous training data
