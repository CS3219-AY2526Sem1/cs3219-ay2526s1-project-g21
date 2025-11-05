package handlers

import (
	"encoding/json"
	"net/http"
	"peerprep/user/internal/models"
	"peerprep/user/internal/repositories"
	"peerprep/user/internal/utils"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/go-chi/chi/v5"
)

type UserHandler struct {
	Repo      UserRepository
	JWTSecret string
	Tokens    *repositories.TokenRepository
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
	sub, err := utils.GetUserIDFromClaims(claims)
	if err != nil || sub != userID {
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

type changeUsernameRequest struct {
	Username string `json:"username"`
}

func (h *UserHandler) ChangeUsernameHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := utils.VerifyToken(r, h.JWTSecret)
	if err != nil {
		utils.JSONError(w, http.StatusUnauthorized, err.Error())
		return
	}
	userID := chi.URLParam(r, "id")
	if sub, err := utils.GetUserIDFromClaims(claims); err != nil || sub != userID {
		utils.JSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	var req changeUsernameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		utils.JSONError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	// Uniqueness with lazy cleanup
	if existing, err := h.Repo.GetUserByUsername(req.Username); err == nil && existing != nil {
		if deleted, _ := repositories.CleanupUnverifiedUserIfExpired(h.Repo.(*repositories.UserRepository), h.Tokens, existing); !deleted {
			utils.JSONError(w, http.StatusConflict, "Username taken")
			return
		}
	}
	user, err := h.Repo.UpdateUser(userID, &models.User{Username: req.Username})
	if err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Failed to change username")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"id": user.ID, "username": user.Username})
}

type changePasswordRequest struct {
	NewPassword     string `json:"newPassword"`
	ConfirmPassword string `json:"confirmPassword"`
}

func (h *UserHandler) ChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := utils.VerifyToken(r, h.JWTSecret)
	if err != nil {
		utils.JSONError(w, http.StatusUnauthorized, err.Error())
		return
	}
	userID := chi.URLParam(r, "id")
	if sub, err := utils.GetUserIDFromClaims(claims); err != nil || sub != userID {
		utils.JSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSONError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	if req.NewPassword == "" || req.NewPassword != req.ConfirmPassword {
		utils.JSONError(w, http.StatusBadRequest, "Passwords do not match")
		return
	}
	if !utils.IsPasswordValid(req.NewPassword) {
		utils.JSONError(w, http.StatusBadRequest, "Password must be at least 8 characters long and include 1 special character")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}
	if _, err := h.Repo.UpdateUser(userID, &models.User{PasswordHash: string(hash)}); err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Failed to change password")
		return
	}
	utils.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

type initiateEmailChangeRequest struct {
	Email string `json:"email"`
}

func (h *UserHandler) InitiateEmailChangeHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := utils.VerifyToken(r, h.JWTSecret)
	if err != nil {
		utils.JSONError(w, http.StatusUnauthorized, err.Error())
		return
	}
	userID := chi.URLParam(r, "id")
	if sub, err := utils.GetUserIDFromClaims(claims); err != nil || sub != userID {
		utils.JSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	var req initiateEmailChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		utils.JSONError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	// Load current user
	current, err := h.Repo.GetUserByID(userID)
	if err != nil {
		utils.JSONError(w, http.StatusNotFound, "User not found")
		return
	}
	if strings.EqualFold(current.Email, req.Email) {
		utils.JSONError(w, http.StatusBadRequest, "Email is unchanged")
		return
	}
	// Uniqueness across existing emails and pending new_email
	if existing, err := h.Repo.GetUserByEmail(req.Email); err == nil && existing != nil {
		if deleted, _ := repositories.CleanupUnverifiedUserIfExpired(h.Repo.(*repositories.UserRepository), h.Tokens, existing); !deleted {
			utils.JSONError(w, http.StatusConflict, "Email taken")
			return
		}
	}
	if _, err := h.Repo.(*repositories.UserRepository).GetUserByNewEmail(req.Email); err == nil {
		utils.JSONError(w, http.StatusConflict, "Email taken")
		return
	}
	// Set new_email and create token
	newEmail := req.Email
	if _, err := h.Repo.UpdateUser(userID, &models.User{NewEmail: &newEmail}); err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Failed to set new email")
		return
	}
	// Create token
	tokenStr, err := generateTokenString(32)
	if err != nil {
		// Revert NewEmail field to nil if token generation fails
		nilEmail := (*string)(nil)
		_, _ = h.Repo.UpdateUser(userID, &models.User{NewEmail: nilEmail})
		utils.JSONError(w, http.StatusInternalServerError, "Failed to generate confirmation token")
		return
	}
	// Clean existing tokens for this purpose
	idU64, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Invalid user ID")
		return
	}
	_ = h.Tokens.DeleteByUserAndPurpose(uint(idU64), models.TokenPurposeEmailChange)
	_ = h.Tokens.Create(&models.Token{
		Token:     tokenStr,
		Purpose:   models.TokenPurposeEmailChange,
		UserID:    uint(idU64),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})
	// Send link to backend confirm endpoint which will redirect
	confirmURL := serverBaseURL() + "/api/v1/auth/change-email/confirm?token=" + tokenStr
	_ = sendEmailAsync(newEmail, "Confirm your new email", "Confirm your new email by visiting: "+confirmURL)
	utils.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
