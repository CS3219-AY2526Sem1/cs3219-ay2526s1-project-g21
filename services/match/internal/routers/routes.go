package routers

import (
	matchManager "match/internal/match_management"

	"github.com/go-chi/chi/v5"
)

func MatchRoutes(r *chi.Mux, mm *matchManager.MatchManager) {
	r.Route("/api/v1/match", func(r chi.Router) {
		r.Post("/join", mm.JoinHandler)
		r.Post("/cancel", mm.CancelHandler)
		r.Get("/check", mm.CheckHandler)
		r.Post("/done", mm.DoneHandler)
		r.Post("/handshake", mm.HandshakeHandler)
		r.Post("/session/feedback", mm.SessionFeedbackHandler)
		r.HandleFunc("/ws", mm.WsHandler)

		r.Options("/join", mm.JoinHandler)
		r.Options("/cancel", mm.CancelHandler)
		r.Options("/check", mm.CheckHandler)
		r.Options("/done", mm.DoneHandler)
		r.Options("/handshake", mm.HandshakeHandler)
		r.Options("/session/feedback", mm.SessionFeedbackHandler)
		r.Options("/ws", mm.WsHandler)
	})
}
