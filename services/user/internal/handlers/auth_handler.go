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
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}

	username := req.Username
	email := req.Email

	if existing, _ := h.Repo.GetUserByUsername(username); existing != nil {
		http.Error(w, "username taken", http.StatusConflict)
		return
	}
	if existing, _ := h.Repo.GetUserByEmail(email); existing != nil {
		http.Error(w, "email taken", http.StatusConflict)
		return
	}

	if !utils.IsPasswordValid(req.Password) {
		http.Error(w, "password must be at least 8 characters long and include 1 special character", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	user := &models.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
	}
	if err := h.Repo.CreateUser(user); err != nil {
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
	})
}

func (h *AuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	username := strings.ToLower(req.Username)

	user, err := h.Repo.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
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
		http.Error(w, "failed to sign token", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(authResponse{Token: signed})
}

func (h *AuthHandler) MeHandler(w http.ResponseWriter, r *http.Request) {
	claims, err := utils.VerifyToken(r, h.JWTSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	uid, err := utils.GetUserIDFromClaims(claims) // <-- safe extraction
	if err != nil {
		http.Error(w, "invalid token subject", http.StatusUnauthorized)
		return
	}

	user, err := h.Repo.GetUserByID(uid)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
	})
}
