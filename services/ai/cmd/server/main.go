package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"peerprep/ai/internal/config"
	"peerprep/ai/internal/feedback"
	"peerprep/ai/internal/handlers"
	"peerprep/ai/internal/jobs"
	"peerprep/ai/internal/llm"
	_ "peerprep/ai/internal/llm/gemini"
	"peerprep/ai/internal/models"
	"peerprep/ai/internal/prompts"
	"peerprep/ai/internal/routers"
	"peerprep/ai/internal/tuning"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func registerRoutes(router *chi.Mux, aiHandler *handlers.AIHandler, feedbackHandler *handlers.FeedbackHandler, modelHandler *handlers.ModelHandler, healthHandler *handlers.HealthHandler) {
	routers.HealthRoutes(router, healthHandler)
	routers.AIRoutes(router, aiHandler, feedbackHandler, modelHandler)
}

// Helper functions for environment variables
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

// initDatabase initializes the PostgreSQL database connection
func initDatabase() (*gorm.DB, error) {
	host := getEnv("POSTGRES_HOST", "localhost")
	user := getEnv("POSTGRES_USER", "postgres")
	password := getEnv("POSTGRES_PASSWORD", "postgres")
	dbname := getEnv("POSTGRES_DB", "postgres")
	port := getEnv("POSTGRES_PORT", "5432")
	sslmode := getEnv("POSTGRES_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		host, user, password, dbname, port, sslmode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate feedback tables
	if err := db.AutoMigrate(&models.AIFeedback{}, &models.ModelVersion{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	logger.Info("Configuration loaded",
		zap.String("provider", cfg.Provider))

	// prompt manager
	promptManager, err := prompts.NewPromptManager()
	if err != nil {
		logger.Fatal("Failed to initialize prompt manager", zap.Error(err))
	}

	// AI provider based on configuration
	aiProvider, err := llm.NewProvider(cfg.Provider)
	if err != nil {
		logger.Fatal("Failed to initialize AI provider", zap.Error(err))
	}

	aiHandler := handlers.NewAIHandler(aiProvider, promptManager, logger)
	healthHandler := handlers.NewHealthHandler(aiProvider, promptManager, cfg)

	// Initialize database for feedback storage
	db, err := initDatabase()
	if err != nil {
		logger.Error("Failed to initialize database, feedback system will be disabled", zap.Error(err))
	}

	// Initialize feedback manager (only if database is available)
	var feedbackManager *feedback.FeedbackManager
	var feedbackHandler *handlers.FeedbackHandler
	var modelHandler *handlers.ModelHandler
	var exporterJob *jobs.FeedbackExporterJob
	var geminiTuner *tuning.GeminiTuner

	if db != nil {
		cacheTTL, _ := time.ParseDuration(getEnv("FEEDBACK_CACHE_TTL", "15m"))
		feedbackManager = feedback.NewFeedbackManager(db, cacheTTL)

		// Set feedback manager on AI handler
		aiHandler.SetFeedbackManager(feedbackManager)

		// Set database connection on AI provider for model selection
		// (only if provider is Gemini)
		if cfg.Provider == "gemini" {
			if geminiClient, ok := aiProvider.(interface{ SetDatabase(*gorm.DB) }); ok {
				geminiClient.SetDatabase(db)
				logger.Info("Database connection set on Gemini provider for A/B testing")
			}
		}

		// Initialize Gemini tuner (for both auto-tuning and manual model management)
		geminiTuner, err = tuning.NewGeminiTuner(
			os.Getenv("GEMINI_API_KEY"),
			getEnv("GEMINI_PROJECT_ID", ""),
			db,
		)
		if err != nil {
			logger.Warn("Failed to initialize Gemini tuner", zap.Error(err))
		}

		// Initialize feedback exporter job
		exporterConfig := &jobs.ExporterConfig{
			Schedule:          getEnv("FEEDBACK_EXPORT_SCHEDULE", "0 2 * * *"),
			ExportDir:         getEnv("FEEDBACK_EXPORT_DIR", "./exports"),
			ExportEnabled:     getEnv("FEEDBACK_EXPORT_ENABLED", "false") == "true",
			AutoTuneEnabled:   getEnv("FEEDBACK_AUTO_TUNE_ENABLED", "false") == "true",
			MinSamplesForTune: getEnvInt("FEEDBACK_MIN_SAMPLES_FOR_TUNE", 100),
			BaseModel:         getEnv("TUNING_BASE_MODEL", "gemini-1.5-flash"),
			LearningRate:      getEnvFloat("TUNING_LEARNING_RATE", 0.001),
			EpochCount:        getEnvInt("TUNING_EPOCH_COUNT", 5),
			AdapterSize:       getEnv("TUNING_ADAPTER_SIZE", "ADAPTER_SIZE_ONE"),
		}

		exporterJob = jobs.NewFeedbackExporterJob(feedbackManager, geminiTuner, exporterConfig)
		if exporterConfig.ExportEnabled {
			if err := exporterJob.Start(); err != nil {
				logger.Error("Failed to start feedback exporter job", zap.Error(err))
			} else {
				logger.Info("Feedback exporter job started", zap.String("schedule", exporterConfig.Schedule))
			}
		}

		// Create feedback handler
		feedbackHandler = handlers.NewFeedbackHandler(feedbackManager)

		// Create model management handler (only if tuner is available)
		if geminiTuner != nil {
			modelHandler = handlers.NewModelHandler(db, geminiTuner)
			logger.Info("Model management endpoints enabled")
		}

		logger.Info("Feedback system initialized successfully")
	}

	router := chi.NewRouter()

	// cors middleware
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "https://d1z9c2graxigrz.cloudfront.net"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	router.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer, middleware.Timeout(60*time.Second))

	registerRoutes(router, aiHandler, feedbackHandler, modelHandler, healthHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	serverAddr := ":" + port

	// http server with timeouts
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// starting server in a goroutine
	go func() {
		logger.Info("AI service starting", zap.String("addr", serverAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// wait for interrupt signal to gracefully shutdown the server
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownChan

	logger.Info("AI service shutting down...")

	// Stop feedback exporter job if running
	if exporterJob != nil {
		exporterJob.Stop()
		logger.Info("Feedback exporter job stopped")
	}

	// graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("AI service exited")
}
