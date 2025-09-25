package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"peerprep/user/internal/handlers"
	"peerprep/user/internal/models"
	"peerprep/user/internal/repositories"
	"peerprep/user/internal/routers"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func connectWithRetry(dsn string, maxWait time.Duration, logger *zap.Logger) (*gorm.DB, error) {
	start := time.Now()
	var lastErr error
	backoff := 500 * time.Millisecond
	for {
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			// Verify connection
			var sqlDB *sql.DB
			sqlDB, err = db.DB()
			if err == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					return db, nil
				}
			}
		}
		lastErr = err
		if time.Since(start) > maxWait {
			return nil, lastErr
		}
		logger.Warn("DB not ready, retrying...", zap.Error(err))
		time.Sleep(backoff)
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Initialize database connection
	dbUser := os.Getenv("POSTGRES_USER")
	dbPass := os.Getenv("POSTGRES_PASSWORD")
	dbName := os.Getenv("POSTGRES_DB")
	dsn := fmt.Sprintf("host=postgres user=%s password=%s dbname=%s port=5432 sslmode=disable",
		dbUser, dbPass, dbName)

	db, err := connectWithRetry(dsn, 30*time.Second, logger)
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

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

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
