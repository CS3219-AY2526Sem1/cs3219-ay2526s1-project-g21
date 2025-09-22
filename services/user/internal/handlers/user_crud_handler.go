package handlers

import (
    "encoding/json"
    "net/http"
    "peerprep/user/internal/models"
    "peerprep/user/internal/repositories"

    "github.com/go-chi/chi/v5"
)

type UserHandler struct {
    Repo *repositories.UserRepository
}

// CreateUserHandler handles user creation
func (h *UserHandler) CreateUserHandler(w http.ResponseWriter, r *http.Request) {
    var user models.User
    if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
        http.Error(w, "Invalid request payload", http.StatusBadRequest)
        return
    }

    // Save user using the repository
    if err := h.Repo.CreateUser(&user); err != nil {
        http.Error(w, "Failed to create user", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(user)
}

// GetUserHandler retrieves a user by ID
func (h *UserHandler) GetUserHandler(w http.ResponseWriter, r *http.Request) {
    userID := chi.URLParam(r, "id")
    if userID == "" {
        http.Error(w, "User ID is required", http.StatusBadRequest)
        return
    }

    user, err := h.Repo.GetUserByID(userID)
    if err != nil {
        if err == repositories.ErrUserNotFound {
            http.Error(w, "User not found", http.StatusNotFound)
        } else {
            http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
        }
        return
    }

    json.NewEncoder(w).Encode(user)
}

// UpdateUserHandler updates user details
func (h *UserHandler) UpdateUserHandler(w http.ResponseWriter, r *http.Request) {
    userID := chi.URLParam(r, "id")
    if userID == "" {
        http.Error(w, "User ID is required", http.StatusBadRequest)
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
    userID := chi.URLParam(r, "id")
    if userID == "" {
        http.Error(w, "User ID is required", http.StatusBadRequest)
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