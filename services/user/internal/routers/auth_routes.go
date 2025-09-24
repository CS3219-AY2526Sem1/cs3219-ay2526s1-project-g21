package routers

import (
	"peerprep/user/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func AuthRoutes(r *chi.Mux, authHandler *handlers.AuthHandler) {
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login", authHandler.LoginHandler)       // User login
		r.Post("/register", authHandler.RegisterHandler) // User registration
		r.Get("/me", authHandler.MeHandler)              // Current user
	})
}