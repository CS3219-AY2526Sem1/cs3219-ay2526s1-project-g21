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
	"peerprep/question/internal/repositories"
	"peerprep/question/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func registerRoutes(router *chi.Mux, logger *zap.Logger, repo *repositories.QuestionRepository) {
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

		questions, err := repo.GetAll()
		if err != nil {
			utils.JSON(resp_writer, http.StatusInternalServerError, models.ErrorResponse{
				Code:    "internal_error",
				Message: "Failed to fetch questions",
			})
			return
		}

		response := models.QuestionsResponse{
			Total: len(questions),
			Items: questions,
		}

		utils.JSON(resp_writer, http.StatusOK, response)
	})

	// create question (stub) - to be used by admin
	// TODO: update this
	router.Post("/questions", func(resp_writer http.ResponseWriter, r *http.Request) {
		resp_writer.Header().Set("Content-Type", "application/json")

		// TODO: needs more error handling and validation
		var question models.Question
		if err := json.NewDecoder(r.Body).Decode(&question); err != nil {
			resp_writer.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(resp_writer).Encode(models.ErrorResponse{
				Code:    "invalid_request",
				Message: "Invalid request payload",
			})
			return
		}

		created, err := repo.Create(&question)
		if err != nil {
			resp_writer.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(resp_writer).Encode(models.ErrorResponse{
				Code:    "internal_error",
				Message: "Failed to create question",
			})
			return
		}

		resp_writer.Header().Set("Location", "/questions/"+created.ID)
		resp_writer.WriteHeader(http.StatusCreated)
		json.NewEncoder(resp_writer).Encode(created)
	})

	// get by id (stub)
	// TODO: update this
	router.Get("/questions/{id}", func(resp_writer http.ResponseWriter, r *http.Request) {
		resp_writer.Header().Set("Content-Type", "application/json")
		id := chi.URLParam(r, "id")

		question, err := repo.GetByID(id)
		if err != nil {
			utils.JSON(resp_writer, http.StatusNotFound, models.ErrorResponse{
				Code:    "question_not_found",
				Message: "Question not found",
			})
			return
		}

		utils.JSON(resp_writer, http.StatusOK, question)
	})

	// update (stub)
	router.Put("/questions/{id}", func(resp_writer http.ResponseWriter, r *http.Request) {
		resp_writer.Header().Set("Content-Type", "application/json")
		id := chi.URLParam(r, "id")

		var question models.Question
		if err := json.NewDecoder(r.Body).Decode(&question); err != nil {
			resp_writer.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(resp_writer).Encode(models.ErrorResponse{
				Code:    "invalid_request",
				Message: "Invalid request payload",
			})
			return
		}

		updated, err := repo.Update(id, &question)
		if err != nil {
			resp_writer.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(resp_writer).Encode(models.ErrorResponse{
				Code:    "internal_error",
				Message: "Failed to update question",
			})
			return
		}

		resp_writer.WriteHeader(http.StatusOK)
		json.NewEncoder(resp_writer).Encode(updated)
	})

	// delete (stub)
	router.Delete("/questions/{id}", func(resp_writer http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		if err := repo.Delete(id); err != nil {
			resp_writer.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(resp_writer).Encode(models.ErrorResponse{
				Code:    "internal_error",
				Message: "Failed to delete question",
			})
			return
		}

		resp_writer.WriteHeader(http.StatusNoContent)
	})

	// random (stub)
	router.Get("/questions/random", func(resp_writer http.ResponseWriter, r *http.Request) {
		// for now we send back 404 to reflect no eligible question in stub
		resp_writer.Header().Set("Content-Type", "application/json")

		_, err := repo.GetRandom()
		if err != nil {
			utils.JSON(resp_writer, http.StatusNotFound, models.ErrorResponse{
				Code:    "no_eligible_question",
				Message: "no eligible question found",
			})
			return
		}
	})
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// initialise repository
	questionRepo := repositories.NewQuestionRepository()

	router := chi.NewRouter()
	router.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer, middleware.Timeout(60*time.Second))

	registerRoutes(router, logger, questionRepo)

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
