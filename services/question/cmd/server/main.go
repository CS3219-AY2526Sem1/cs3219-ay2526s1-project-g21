package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type Question struct {
	Number        int       `json:"number"`                   // unique, auto-assigned human-facing id
	Title         string    `json:"title"`                    // short title (<= 200 chars)
	PromptMd      string    `json:"prompt_md"`                // full problem statement in Markdown (<= 20k chars)
	Examples      []Example `json:"examples,omitempty"`       // optional LeetCode-style I/O examples
	ConstraintsMd string    `json:"constraints_md,omitempty"` // optional constraints in Markdown (<= 5k chars)
	TopicTags     []string  `json:"topic_tags,omitempty"`     // optional free-form tags (≤10 items, each ≤50 chars)
	Difficulty    string    `json:"difficulty"`               // enum: "Easy" | "Medium" | "Hard"
}

// represents a LeetCode-style I/O example
type Example struct {
	InputMd       string `json:"input_md"`       // sample input in Markdown
	OutputMd      string `json:"output_md"`      // expected output in Markdown
	ExplanationMd string `json:"explanation_md"` // reasoning in Markdown
}

// represents the response structure for /questions endpoint, all questions sent for now.
// TODO: implement other question fetching endpoints
type QuestionsResponse struct {
	Total int        `json:"total"`
	Items []Question `json:"items"`
}

func registerRoutes(router *chi.Mux, logger *zap.Logger) {
	// Health check endpoints
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	// questions endpoint - walking skeleton returns empty list
	router.Get("/questions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := QuestionsResponse{
			Total: 0,
			Items: []Question{},
		}

		json.NewEncoder(w).Encode(response)
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

	// Wait for interrupt signal to gracefully shutdown the server
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownChan

	logger.Info("Question service shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("Question service exited")
}
