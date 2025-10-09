package routers

import (
	"peerprep/question/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func QuestionRoutes(router *chi.Mux, questionHandler *handlers.QuestionHandler) {
	router.Get("/questions", questionHandler.GetQuestionsHandler)
	router.Post("/questions", questionHandler.CreateQuestionHandler)
	router.Get("/questions/{id}", questionHandler.GetQuestionByIDHandler)
	router.Put("/questions/{id}", questionHandler.UpdateQuestionHandler)
	router.Delete("/questions/{id}", questionHandler.DeleteQuestionHandler)
	router.Get("/questions/random", questionHandler.GetRandomQuestionHandler)
}
