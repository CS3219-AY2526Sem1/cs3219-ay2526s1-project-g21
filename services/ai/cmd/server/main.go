package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"peerprep/ai/internal/config"
	"peerprep/ai/internal/handlers"
	"peerprep/ai/internal/llm"
	_ "peerprep/ai/internal/llm/gemini"
	"peerprep/ai/internal/prompts"
	"peerprep/ai/internal/routers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
)

func registerRoutes(router *chi.Mux, aiHandler *handlers.AIHandler, healthHandler *handlers.HealthHandler) {
	routers.HealthRoutes(router, healthHandler)
	routers.AIRoutes(router, aiHandler)
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

	defer func() {
		if closer, ok := aiProvider.(llm.Closer); ok {
			if err := closer.Close(); err != nil {
				logger.Error("Failed to close AI provider", zap.Error(err))
			}
		}
	}()

	aiHandler := handlers.NewAIHandler(aiProvider, promptManager, logger)
	healthHandler := handlers.NewHealthHandler()

	router := chi.NewRouter()

	// cors middleware
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	router.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer, middleware.Timeout(60*time.Second))

	registerRoutes(router, aiHandler, healthHandler)

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

	// graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("AI service exited")
}
