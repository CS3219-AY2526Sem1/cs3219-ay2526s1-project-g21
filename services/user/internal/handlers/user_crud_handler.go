package handlers

import (
	"encoding/json"
	"net/http"
	"peerprep/user/internal/models"
	"peerprep/user/internal/repositories"
	"peerprep/user/internal/utils"

	"github.com/go-chi/chi/v5"
)

type UserHandler struct {
	Repo      *repositories.UserRepository
	JWTSecret string
}

// UpdateUserHandler updates user details
func (h *UserHandler) UpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := utils.VerifyToken(r, h.JWTSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Only allow user to update their own record
	if sub, ok := claims["sub"].(string); !ok || sub != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var updates models.User
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	user, err := h.Repo.UpdateUser(userID, &updates)
	if err != nil {
		if err == repositories.ErrUserNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to update user", http.StatusInternalServerError)
		}
		return
	}

	json.NewEncoder(w).Encode(user)
}

// DeleteUserHandler deletes a user by ID
func (h *UserHandler) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := utils.VerifyToken(r, h.JWTSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Only allow user to delete their own record
	if sub, ok := claims["sub"].(string); !ok || sub != userID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := h.Repo.DeleteUser(userID); err != nil {
		if err == repositories.ErrUserNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
