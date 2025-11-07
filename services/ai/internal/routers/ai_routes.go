package routers

import (
	"peerprep/ai/internal/handlers"
	"peerprep/ai/internal/middleware"
	"peerprep/ai/internal/models"

	"github.com/go-chi/chi/v5"
)

func AIRoutes(router *chi.Mux, aiHandler *handlers.AIHandler) {
	router.Route("/api/v1/ai", func(r chi.Router) {
		r.With(middleware.ValidateRequest[*models.ExplainRequest]()).Post("/explain", aiHandler.ExplainHandler)
		r.With(middleware.ValidateRequest[*models.HintRequest]()).Post("/hint", aiHandler.HintHandler)
	})
}
