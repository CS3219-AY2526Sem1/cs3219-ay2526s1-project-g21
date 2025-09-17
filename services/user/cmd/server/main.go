package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

func registerRoutes(r *chi.Mux, logger *zap.Logger) {
	// F1.* stubs
	r.Post("/api/v1/auth/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"registered_stub"}`))
	})
	r.Post("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"accessToken":"stub","refreshToken":"stub"}`))
	})
	r.Post("/api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"logged_out_stub"}`))
	})
	r.Put("/api/v1/account", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"account_updated_stub"}`))
	})
	r.Delete("/api/v1/account", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"account_deleted_stub"}`))
	})
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer, middleware.Timeout(60*time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	registerRoutes(r, logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("user-svc listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
