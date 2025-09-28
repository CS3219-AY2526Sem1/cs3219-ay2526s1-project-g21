package routers

import (
	handlers "peerprep/user/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func UserRoutes(r *chi.Mux, userHandler *handlers.UserHandler) {
	r.Route("/api/v1/users", func(r chi.Router) {
		r.Put("/{id}", userHandler.UpdateUserHandler)    // Update user by ID
		r.Delete("/{id}", userHandler.DeleteUserHandler) // Delete user by ID
	})
}
