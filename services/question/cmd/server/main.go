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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func registerRoutes(router *chi.Mux, logger *zap.Logger, questionHandler *handlers.QuestionHandler, healthHandler *handlers.HealthHandler) {
	// Health check endpoints
	router.Get("/healthz", healthHandler.HealthzHandler)
	router.Get("/readyz", healthHandler.ReadyzHandler)

	// Question endpoints
	router.Get("/questions", questionHandler.GetQuestionsHandler)
	router.Post("/questions", questionHandler.CreateQuestionHandler)
	router.Get("/questions/{id}", questionHandler.GetQuestionByIDHandler)
	router.Put("/questions/{id}", questionHandler.UpdateQuestionHandler)
	router.Delete("/questions/{id}", questionHandler.DeleteQuestionHandler)
	router.Get("/questions/random", questionHandler.GetRandomQuestionHandler)
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// initialise repository
	questionRepo := repositories.NewQuestionRepository()

	// initialise handlers
	questionHandler := handlers.NewQuestionHandler(questionRepo)
	healthHandler := handlers.NewHealthHandler()

	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer, middleware.Timeout(60*time.Second))

	registerRoutes(router, logger, questionHandler, healthHandler)

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
