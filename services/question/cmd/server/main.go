package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"peerprep/question/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)


func registerRoutes(router *chi.Mux, logger *zap.Logger) {
	// Health check endpoints
	router.Get("/healthz", func(resp_writer http.ResponseWriter, r *http.Request) {
		resp_writer.WriteHeader(http.StatusOK)
		resp_writer.Write([]byte("ok"))
	})

	router.Get("/readyz", func(resp_writer http.ResponseWriter, r *http.Request) {
		resp_writer.WriteHeader(http.StatusOK)
		resp_writer.Write([]byte("ready"))
	})

	// questions endpoint - walking skeleton returns empty list
	router.Get("/questions", func(resp_writer http.ResponseWriter, r *http.Request) {
		resp_writer.Header().Set("Content-Type", "application/json")
		resp_writer.WriteHeader(http.StatusOK)

		response := models.QuestionsResponse{
			Total: 0,
			Items: []models.Question{},
		}

		json.NewEncoder(resp_writer).Encode(response)
	})

	// create question (stub) - to be used by admin
	// TODO: update this
	router.Post("/questions", func(resp_writer http.ResponseWriter, r *http.Request) {
		resp_writer.Header().Set("Content-Type", "application/json")
		resp_writer.Header().Set("Location", "/questions/stub-id")
		resp_writer.WriteHeader(http.StatusCreated)

		current_time := time.Now().UTC()
		resp := models.Question{
			ID:             "stub-id",
			Title:          "stub",
			Difficulty:     models.Easy,
			TopicTags:      []string{"Stub"},
			PromptMarkdown: "stub prompt",
			Constraints:    "",
			TestCases:      []models.TestCase{{Input: "1", Output: "1"}},
			Status:         models.StatusActive,
			Author:         "system",
			CreatedAt:      current_time,
			UpdatedAt:      current_time,
		}
		json.NewEncoder(resp_writer).Encode(resp)
	})

	// get by id (stub)
	// TODO: update this
	router.Get("/questions/{id}", func(resp_writer http.ResponseWriter, r *http.Request) {
		resp_writer.Header().Set("Content-Type", "application/json")
		id := chi.URLParam(r, "id")
		current_time := time.Now().UTC()
		resp := models.Question{
			ID:             id,
			Title:          "stub",
			Difficulty:     models.Medium,
			TopicTags:      []string{"Stub"},
			PromptMarkdown: "stub prompt",
			Constraints:    "",
			TestCases:      []models.TestCase{{Input: "1", Output: "1"}},
			Status:         models.StatusActive,
			Author:         "system",
			CreatedAt:      current_time,
			UpdatedAt:      current_time,
		}
		json.NewEncoder(resp_writer).Encode(resp)
	})

	// update (stub)
	router.Put("/questions/{id}", func(resp_writer http.ResponseWriter, r *http.Request) {
		resp_writer.Header().Set("Content-Type", "application/json")
		id := chi.URLParam(r, "id")
		current_time := time.Now().UTC()
		resp := models.Question{
			ID:             id,
			Title:          "stub-updated",
			Difficulty:     models.Hard,
			TopicTags:      []string{"Stub"},
			PromptMarkdown: "stub prompt updated",
			Constraints:    "",
			TestCases:      []models.TestCase{{Input: "1", Output: "1"}},
			Status:         models.StatusActive,
			Author:         "system",
			CreatedAt:      current_time.Add(-time.Hour),
			UpdatedAt:      current_time,
		}
		json.NewEncoder(resp_writer).Encode(resp)
	})

	// delete (stub)
	router.Delete("/questions/{id}", func(resp_writer http.ResponseWriter, r *http.Request) {
		resp_writer.WriteHeader(http.StatusNoContent)
	})

	// random (stub)
	router.Get("/questions/random", func(resp_writer http.ResponseWriter, r *http.Request) {
		// for now we send back 404 to reflect no eligible question in stub
		resp_writer.Header().Set("Content-Type", "application/json")
		resp_writer.WriteHeader(http.StatusNotFound)
		json.NewEncoder(resp_writer).Encode(models.ErrorResponse{
			Code:    "no_eligible_question",
			Message: "no eligible question found",
		})
	})
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer, middleware.Timeout(60*time.Second))

	registerRoutes(router, logger)

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
