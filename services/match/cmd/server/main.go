package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func registerRoutes(r *chi.Mux, logger *zap.Logger) {
	// F2.* stubs
	r.Post("/api/v1/match/request", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"status":"enqueued_stub"}`))
	})
	r.Delete("/api/v1/match/request", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"cancelled_stub"}`))
	})
	r.Get("/api/v1/match/counters", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"waiting":0}`))
	})
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.WriteJSON(map[string]any{"type": "waiting_count_update", "waiting": 0})
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
	log.Printf("match-svc listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}
