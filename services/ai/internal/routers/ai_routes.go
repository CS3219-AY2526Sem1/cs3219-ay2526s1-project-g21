package routers

import (
	"peerprep/ai/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func AIRoutes(router *chi.Mux, aiHandler *handlers.AIHandler) {
	router.Route("/ai", func(r chi.Router) {
		r.Post("/explain", aiHandler.ExplainHandler)
		// future routes for hint mode
		// r.Post("/hint", aiHandler.HintHandler)
	})
}
