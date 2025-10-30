package routers

import (
	"peerprep/ai/internal/handlers"
	"peerprep/ai/internal/middleware"
	"peerprep/ai/internal/models"

	"github.com/go-chi/chi/v5"
)

func AIRoutes(router *chi.Mux, aiHandler *handlers.AIHandler) {
	router.Route("/ai", func(r chi.Router) {
		r.With(middleware.ValidateRequest[*models.ExplainRequest]()).Post("/explain", aiHandler.ExplainHandler)
		// future routes for hint mode
		// r.Post("/hint", aiHandler.HintHandler)
	})
}
