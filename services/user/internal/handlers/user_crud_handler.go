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
	Repo      UserRepository
	JWTSecret string
}

// UpdateUserHandler updates user details
func (h *UserHandler) UpdateUserHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := utils.VerifyToken(r, h.JWTSecret)
	if err != nil {
		utils.JSONError(w, http.StatusUnauthorized, err.Error())
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		utils.JSONError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	// Only allow user to update their own record
	if sub, ok := claims["sub"].(string); !ok || sub != userID {
		utils.JSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var updates models.User
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		utils.JSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	user, err := h.Repo.UpdateUser(userID, &updates)
	if err != nil {
		if err == repositories.ErrUserNotFound {
			utils.JSONError(w, http.StatusNotFound, "User not found")
		} else {
			utils.JSONError(w, http.StatusInternalServerError, "Failed to update user")
		}
		return
	}

	utils.JSON(w, http.StatusOK, user)
}

// DeleteUserHandler deletes a user by ID
func (h *UserHandler) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := utils.VerifyToken(r, h.JWTSecret)
	if err != nil {
		utils.JSONError(w, http.StatusUnauthorized, err.Error())
		return
	}

	userID := chi.URLParam(r, "id")
	if userID == "" {
		utils.JSONError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	// Only allow user to delete their own record
	if sub, ok := claims["sub"].(string); !ok || sub != userID {
		utils.JSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	if err := h.Repo.DeleteUser(userID); err != nil {
		if err == repositories.ErrUserNotFound {
			utils.JSONError(w, http.StatusNotFound, "User not found")
		} else {
			utils.JSONError(w, http.StatusInternalServerError, "Failed to delete user")
		}
		return
	}
}
