package routers

import (
	"peerprep/question/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func QuestionRoutes(r *chi.Mux, questionHandler *handlers.QuestionHandler, healthHandler *handlers.HealthHandler) {
	r.Route("/api/v1/questions", func(r chi.Router) {
		r.Get("/", questionHandler.GetQuestionsHandler)
		r.Post("/", questionHandler.CreateQuestionHandler)
		r.Get("/{id}", questionHandler.GetQuestionByIDHandler)
		r.Put("/{id}", questionHandler.UpdateQuestionHandler)
		r.Delete("/{id}", questionHandler.DeleteQuestionHandler)
		r.Get("/random", questionHandler.GetRandomQuestionHandler)

		r.Get("/healthz", healthHandler.HealthzHandler)
		r.Get("/readyz", healthHandler.ReadyzHandler)
	})
}
