package main

import (
	"log"
	"net/http"
	"os"
	"peerprep/user/internal/handlers"
	"peerprep/user/internal/repositories"
	"peerprep/user/internal/routers"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize database connection
	dsn := "host=localhost user=your_user password=your_password dbname=your_db port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Fatal("Failed to connect to the database", zap.Error(err))
	}

	// Initialize repository and handler
	userRepo := &repositories.UserRepository{DB: db}
	userHandler := &handlers.UserHandler{Repo: userRepo}

	// Set up router
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer, middleware.Timeout(60*time.Second))

	// Health check route
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	// Register user routes
	routers.UserRoutes(r, userHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("user-svc listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
