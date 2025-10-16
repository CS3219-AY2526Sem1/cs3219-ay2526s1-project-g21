package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"collab/internal/routers"
	"collab/internal/services"
	"collab/internal/utils"
)

func main() {
	logger := utils.NewLogger()

	// Initialize match service
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	questionURL := os.Getenv("QUESTION_SERVICE_URL")
	if questionURL == "" {
		questionURL = "http://question-service:8080"
	}

	matchService := services.NewMatchService(redisAddr, questionURL)

	// Start Redis subscription in background
	go matchService.SubscribeToMatches()

	r := chi.NewRouter()
	r.Use(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Logger,
		middleware.Recoverer,
		middleware.Timeout(60*time.Second),
	)

	r.Mount("/", routers.New(logger, matchService))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("collab-svc listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
