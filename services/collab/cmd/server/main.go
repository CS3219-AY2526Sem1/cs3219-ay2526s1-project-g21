package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"collab/internal/room_management"
	"collab/internal/routers"
	"collab/internal/utils"
)

var (
	listenAndServe     = http.ListenAndServe
	exitFunc           = defaultExit
	defaultRedisAddr   = "redis:6379"
	defaultQuestionURL = "http://localhost:8082"
	defaultPort        = "8080"
	exit               = os.Exit
)

func defaultExit(err error) {
	log.Println(err)
	exit(1)
}

func main() {
	if err := run(context.Background()); err != nil {
		exitFunc(err)
	}
}

func run(parent context.Context) error {
	logger := utils.NewLogger()

	ctx := parent
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Initialize match service
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = defaultRedisAddr
	}

	questionURL := os.Getenv("QUESTION_SERVICE_URL")
	if questionURL == "" {
		questionURL = defaultQuestionURL
	}

	roomManager := room_management.NewRoomManager(redisAddr, questionURL)

	// Start Redis subscription in background
	go roomManager.SubscribeToMatches(ctx)

	r := chi.NewRouter()
	r.Use(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Logger,
		middleware.Recoverer,
		middleware.Timeout(60*time.Second),
	)

	// CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	r.Mount("/", routers.New(logger, roomManager))

	r.Get("/healthz", healthHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}
	addr := ":" + port
	log.Printf("collab-svc listening on %s", addr)
	return listenAndServe(addr, r)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }
