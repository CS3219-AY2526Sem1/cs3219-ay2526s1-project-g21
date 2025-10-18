package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"peerprep/user/internal/models"
	"peerprep/user/internal/repositories"
	"peerprep/user/internal/utils"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler manages authentication endpoints.
type AuthHandler struct {
	Repo      *repositories.UserRepository
	JWTSecret string
}

func NewAuthHandler(repo *repositories.UserRepository) *AuthHandler {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev"
	}
	return &AuthHandler{Repo: repo, JWTSecret: secret}
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
}

func (h *AuthHandler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSONError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		utils.JSONError(w, http.StatusBadRequest, "Missing fields")
		return
	}

	// Check username/email existence
	if existing, err := h.Repo.GetUserByUsername(req.Username); err != nil && err != repositories.ErrUserNotFound {
		utils.JSONError(w, http.StatusInternalServerError, "Database error checking username")
		return
	} else if existing != nil {
		utils.JSONError(w, http.StatusConflict, "Username taken")
		return
	}

	if existing, err := h.Repo.GetUserByEmail(req.Email); err != nil && err != repositories.ErrUserNotFound {
		utils.JSONError(w, http.StatusInternalServerError, "Database error checking email")
		return
	} else if existing != nil {
		utils.JSONError(w, http.StatusConflict, "Email taken")
		return
	}

	if !utils.IsPasswordValid(req.Password) {
		utils.JSONError(w, http.StatusBadRequest, "Password must be at least 8 characters long and include 1 special character")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
	}

	if err := h.Repo.CreateUser(user); err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	utils.JSON(w, http.StatusCreated, map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
	})
}

func (h *AuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSONError(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	username := strings.ToLower(req.Username)
	user, err := h.Repo.GetUserByUsername(username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		utils.JSONError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Failed to sign token")
		return
	}

	utils.JSON(w, http.StatusOK, authResponse{Token: signed})
}

func (h *AuthHandler) MeHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := utils.VerifyToken(r, h.JWTSecret)
	if err != nil {
		utils.JSONError(w, http.StatusUnauthorized, err.Error())
		return
	}

	uid, err := utils.GetUserIDFromClaims(claims)
	if err != nil {
		utils.JSONError(w, http.StatusUnauthorized, "Invalid token subject")
		return
	}

	user, err := h.Repo.GetUserByID(uid)
	if err != nil {
		utils.JSONError(w, http.StatusNotFound, "User not found")
		return
	}

	utils.JSON(w, http.StatusOK, map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
	})
}
