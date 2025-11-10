package routers

import (
	"peerprep/ai/internal/handlers"
	"peerprep/ai/internal/middleware"
	"peerprep/ai/internal/models"

	"github.com/go-chi/chi/v5"
)

func AIRoutes(router *chi.Mux, aiHandler *handlers.AIHandler, feedbackHandler *handlers.FeedbackHandler) {
	router.Route("/api/v1/ai", func(r chi.Router) {
		// AI generation endpoints
		r.With(middleware.ValidateRequest[*models.ExplainRequest]()).Post("/explain", aiHandler.ExplainHandler)
		r.With(middleware.ValidateRequest[*models.HintRequest]()).Post("/hint", aiHandler.HintHandler)
		r.With(middleware.ValidateRequest[*models.TestGenRequest]()).Post("/tests", aiHandler.TestsHandler)
		r.With(middleware.ValidateRequest[*models.RefactorTipsRequest]()).Post("/refactor-tips", aiHandler.RefactorTipsHandler)

		// Feedback endpoints
		r.Post("/feedback/{request_id}", feedbackHandler.SubmitFeedback)
		r.Get("/feedback/export", feedbackHandler.ExportFeedback)
		r.Get("/feedback/stats", feedbackHandler.GetFeedbackStats)
	})
}
