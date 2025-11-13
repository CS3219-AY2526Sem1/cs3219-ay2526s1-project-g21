package routers

import (
	"peerprep/ai/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func HealthRoutes(router *chi.Mux, healthHandler *handlers.HealthHandler) {
	router.Get("/healthz", healthHandler.HealthzHandler)
	router.Get("/readyz", healthHandler.ReadyzHandler)
	router.Get("/api/v1/ai/healthz", healthHandler.HealthzHandler)
}
