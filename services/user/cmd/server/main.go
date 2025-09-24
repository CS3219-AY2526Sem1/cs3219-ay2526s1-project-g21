package main

import (
	"log"
	"net/http"
	"os"
	"peerprep/user/internal/handlers"
	"peerprep/user/internal/models"
	"peerprep/user/internal/repositories"
	"peerprep/user/internal/routers"
	"time"
	"fmt"

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
	dbUser := os.Getenv("POSTGRES_USER")
	dbPass := os.Getenv("POSTGRES_PASSWORD")
	dbName := os.Getenv("POSTGRES_DB")
	dsn := fmt.Sprintf("host=postgres user=%s password=%s dbname=%s port=5432 sslmode=disable",
		dbUser, dbPass, dbName)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Fatal("Failed to connect to the database", zap.Error(err))
	}

	// Auto-migrate models
	if err := db.AutoMigrate(&models.User{}); err != nil {
		logger.Fatal("Failed to migrate database", zap.Error(err))
	}

	// Initialize repository and handlers
	userRepo := &repositories.UserRepository{DB: db}
	userHandler := &handlers.UserHandler{Repo: userRepo}
	authHandler := handlers.NewAuthHandler(userRepo)

	// Set up router
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer, middleware.Timeout(60*time.Second))

	// Health check route
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	// Register routes
	routers.UserRoutes(r, userHandler)
	routers.AuthRoutes(r, authHandler)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("user-svc listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
