package routers

import (
	handlers "peerprep/user/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func UserRoutes(r *chi.Mux, userHandler *handlers.UserHandler) {
	r.Route("/api/v1/users", func(r chi.Router) {
		r.Post("/", userHandler.CreateUserHandler)   // Create user
		r.Get("/", userHandler.GetUserHandler)       // Get user by ID
		r.Put("/", userHandler.UpdateUserHandler)    // Update user
		r.Delete("/", userHandler.DeleteUserHandler) // Delete user
	})
}
