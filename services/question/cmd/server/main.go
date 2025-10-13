package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"peerprep/question/internal/handlers"
	"peerprep/question/internal/repositories"
	"peerprep/question/internal/routers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
)

func registerRoutes(router *chi.Mux, questionHandler *handlers.QuestionHandler, healthHandler *handlers.HealthHandler) {
	routers.HealthRoutes(router, healthHandler)
	routers.QuestionRoutes(router, questionHandler)
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// initialise repository
	ctx := context.Background()
	questionRepo, err := repositories.NewQuestionRepository(ctx) // Updated
	if err != nil {
		logger.Fatal("failed to init mongo repo", zap.Error(err))
	}

	// initialise handlers
	questionHandler := handlers.NewQuestionHandler(questionRepo)
	healthHandler := handlers.NewHealthHandler()

	router := chi.NewRouter()

	// CORS middleware
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	router.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer, middleware.Timeout(60*time.Second))

	registerRoutes(router, questionHandler, healthHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	serverAddr := ":" + port

	// HTTP server with timeouts
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// strting server in a goroutine
	go func() {
		logger.Info("Question service starting", zap.String("addr", serverAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// wait for interrupt signal to gracefully shutdown the server
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownChan

	logger.Info("Question service shutting down...")

	// graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("Question service exited")
}
