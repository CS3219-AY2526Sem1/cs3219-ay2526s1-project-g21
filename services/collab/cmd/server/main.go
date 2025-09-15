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
	// F4.* stubs
	r.Post("/api/v1/session/start", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"session_started_stub"}`))
	})
	r.Post("/api/v1/session/end", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"session_ended_stub"}`))
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
	log.Printf("collab-svc listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
