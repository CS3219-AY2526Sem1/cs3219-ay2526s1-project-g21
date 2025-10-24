package routers

import (
	"peerprep/question/internal/handlers"

	"github.com/go-chi/chi/v5"
)

// Currently not used as QuestionRoutes takes in both the main question handler and the health handler
func HealthRoutes(r *chi.Mux, healthHandler *handlers.HealthHandler) {
	r.Route("/api/v1/questions", func(r chi.Router) {
		r.Get("/healthz", healthHandler.HealthzHandler)
		r.Get("/readyz", healthHandler.ReadyzHandler)
	})
}
