package routers

import (
	handlers "peerprep/user/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func HistoryRoutes(r *chi.Mux, historyHandler *handlers.HistoryHandler) {
	r.Route("/api/history", func(r chi.Router) {
		r.Get("/{userId}", historyHandler.GetUserHistory)              // Get user's interview history
		r.Get("/{userId}/{matchId}", historyHandler.GetSessionDetails) // Get specific session details
	})
}
