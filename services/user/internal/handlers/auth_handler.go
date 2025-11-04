package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"peerprep/user/internal/models"
	"peerprep/user/internal/repositories"
	"peerprep/user/internal/utils"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var signJWT = func(token *jwt.Token, secret string) (string, error) {
	return token.SignedString([]byte(secret))
}

var generatePasswordHash = bcrypt.GenerateFromPassword

// AuthHandler manages authentication endpoints.
type AuthHandler struct {
	Repo      UserRepository
	JWTSecret string
	Tokens    *repositories.TokenRepository
}

func NewAuthHandler(repo UserRepository, tokens *repositories.TokenRepository) *AuthHandler {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev"
	}
	return &AuthHandler{Repo: repo, JWTSecret: secret, Tokens: tokens}
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

type forgotRequest struct {
	Email string `json:"email"`
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

	// Check username/email existence with lazy cleanup for expired unverified accounts (use Exists to avoid noisy logs)
	if exists, err := h.Repo.(*repositories.UserRepository).ExistsByUsername(req.Username); err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Database error checking username")
		return
	} else if exists {
		if existing, err := h.Repo.GetUserByUsername(req.Username); err == nil && existing != nil {
			if deleted, _ := repositories.CleanupUnverifiedUserIfExpired(h.Repo.(*repositories.UserRepository), h.Tokens, existing); deleted {
				// proceed after cleanup
			} else {
				utils.JSONError(w, http.StatusConflict, "Username taken")
				return
			}
		}
	}

	if exists, err := h.Repo.(*repositories.UserRepository).ExistsByEmail(req.Email); err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Database error checking email")
		return
	} else if exists {
		if existing, err := h.Repo.GetUserByEmail(req.Email); err == nil && existing != nil {
			if deleted, _ := repositories.CleanupUnverifiedUserIfExpired(h.Repo.(*repositories.UserRepository), h.Tokens, existing); deleted {
				// proceed after cleanup
			} else {
				utils.JSONError(w, http.StatusConflict, "Email taken")
				return
			}
		}
	}

	// Also disallow if any valid user's pending new_email equals requested email
	if exists, err := h.Repo.(*repositories.UserRepository).ExistsByNewEmail(req.Email); err == nil && exists {
		utils.JSONError(w, http.StatusConflict, "Email taken")
		return
	}

	if !utils.IsPasswordValid(req.Password) {
		utils.JSONError(w, http.StatusBadRequest, "Password must be at least 8 characters long and include 1 special character")
		return
	}

	hash, err := generatePasswordHash([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	user := &models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
		Verified:     false,
	}

	if err := h.Repo.CreateUser(user); err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	// Create verification token
	tokenStr, err := generateTokenString(32)
	if err == nil {
		_ = h.Tokens.DeleteByUserAndPurpose(user.ID, models.TokenPurposeAccountVerification)
		_ = h.Tokens.Create(&models.Token{
			Token:     tokenStr,
			Purpose:   models.TokenPurposeAccountVerification,
			UserID:    user.ID,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		})
		// Send verification email (implemented in SMTP util task)
		// Send link directly to backend which redirects to frontend for better reliability
		verifyURL := serverBaseURL() + "/api/v1/auth/verify?token=" + tokenStr
		_ = sendEmailAsync(user.Email, "Verify your PeerPrep account", "Please verify your account by visiting: "+verifyURL)
	}

	utils.JSON(w, http.StatusCreated, map[string]any{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"message":  "Registration successful. Please verify your email before logging in.",
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

	// Block login if not verified; lazy cleanup on expiry
	if !user.Verified {
		// Check token expiry
		var t *models.Token
		if h.Tokens != nil {
			if tok, e := h.Tokens.GetByUserAndPurpose(user.ID, models.TokenPurposeAccountVerification); e == nil {
				t = tok
			}
		}
		// If no token or expired, delete user and token
		if t == nil || time.Now().After(t.ExpiresAt) {
			if t != nil {
				_ = h.Tokens.DeleteByID(t.ID)
			}
			_ = h.Repo.DeleteUser(strconv.FormatUint(uint64(user.ID), 10))
			utils.JSONError(w, http.StatusNotFound, "Account not found")
			return
		}
		utils.JSONError(w, http.StatusForbidden, "Please verify your email before logging in")
		return
	}

	claims := jwt.MapClaims{
		"sub":      user.ID,
		"username": user.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := signJWT(token, h.JWTSecret)
	if err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Failed to sign token")
		return
	}

	utils.JSON(w, http.StatusOK, authResponse{Token: signed})
}

// ForgotPasswordHandler sends the username and a newly generated temporary password
// to the user's email, and updates the stored password to the generated one.
// Responds with 200 even if the email is not found to avoid user enumeration.
func (h *AuthHandler) ForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req forgotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSONError(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		utils.JSONError(w, http.StatusBadRequest, "Email is required")
		return
	}

	// Always return 200 at the end
	defer func() {
		utils.JSON(w, http.StatusOK, map[string]any{"ok": true})
	}()

	user, err := h.Repo.GetUserByEmail(email)
	if err != nil {
		return
	}

	// Generate a password that passes validation policy
	tempPwd := generateCompliantPassword()
	hash, err := bcrypt.GenerateFromPassword([]byte(tempPwd), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	_, err = h.Repo.UpdateUser(strconv.FormatUint(uint64(user.ID), 10), &models.User{PasswordHash: string(hash)})
	if err != nil {
		return
	}

	// Email the username and the temporary password
	subject := "Your PeerPrep account recovery"
	body := "Hello,\n\n" +
		"Here are your account details:\n" +
		"Username: " + user.Username + "\n" +
		"Temporary password: " + tempPwd + "\n\n" +
		"You can log in with this password. Consider changing it later in Account settings.\n\n" +
		"If you did not request this, you can ignore this email."
	_ = sendEmailAsync(user.Email, subject, body)
}

// generateCompliantPassword creates a random password >= 12 chars including at least one special char
func generateCompliantPassword() string {
	// Base64 provides letters/numbers/-/_; ensure a special char is present
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback
		return "Tmp!" + base64.RawURLEncoding.EncodeToString([]byte(time.Now().String()))[:12]
	}
	s := base64.RawURLEncoding.EncodeToString(b)
	// Inject at least one special character to satisfy policy
	specials := []rune("!@#$%^&*()-_=+[]{}:;,.?/")
	if !strings.ContainsAny(s, string(specials)) {
		s = s + "!"
	}
	if len(s) < 12 {
		s = s + "#A1"
	}
	return s[:12]
}

func generateTokenString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func clientBaseURL() string {
	if v := os.Getenv("CLIENT_BASE_URL"); v != "" {
		return v
	}
	return "http://localhost:5173"
}

func serverBaseURL() string {
	if v := os.Getenv("USER_SERVICE_BASE_URL"); v != "" {
		return v
	}
	// default local docker compose mapping
	return "http://localhost:8081"
}

// sendEmailAsync delegates to the utils SMTP sender when configured
var sendEmailAsync = func(to, subject, body string) error {
	return utils.SendEmail(to, subject, body)
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

// VerifyAccountHandler marks a user as verified if the token is valid and unexpired.
func (h *AuthHandler) VerifyAccountHandler(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		utils.JSONError(w, http.StatusBadRequest, "Missing token")
		return
	}

	t, err := h.Tokens.GetByToken(tokenStr)
	if err != nil {
		utils.JSONError(w, http.StatusNotFound, "Invalid token")
		return
	}
	if t.Purpose != models.TokenPurposeAccountVerification {
		utils.JSONError(w, http.StatusBadRequest, "Invalid token purpose")
		return
	}
	if time.Now().After(t.ExpiresAt) {
		// Token expired: delete user and token
		_ = h.Tokens.DeleteByID(t.ID)
		// Attempt to delete user if still unverified
		uid := strconv.FormatUint(uint64(t.UserID), 10)
		user, err := h.Repo.GetUserByID(uid)
		if err == nil && !user.Verified {
			_ = h.Repo.DeleteUser(uid)
		}
		utils.JSONError(w, http.StatusGone, "Verification token expired")
		return
	}

	// Mark user verified
	uid := strconv.FormatUint(uint64(t.UserID), 10)
	user, err := h.Repo.GetUserByID(uid)
	if err != nil {
		utils.JSONError(w, http.StatusNotFound, "User not found")
		return
	}
	if !user.Verified {
		user.Verified = true
		if _, err := h.Repo.UpdateUser(uid, &models.User{Verified: true}); err != nil {
			utils.JSONError(w, http.StatusInternalServerError, "Failed to verify user")
			return
		}
	}
	_ = h.Tokens.DeleteByID(t.ID)
	// If reached via browser, redirect to client page
	http.Redirect(w, r, clientBaseURL()+"/verifyaccount?status=ok", http.StatusSeeOther)
}

// ConfirmEmailChangeHandler applies a pending new_email if token valid.
func (h *AuthHandler) ConfirmEmailChangeHandler(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		utils.JSONError(w, http.StatusBadRequest, "Missing token")
		return
	}
	t, err := h.Tokens.GetByToken(tokenStr)
	if err != nil {
		utils.JSONError(w, http.StatusNotFound, "Invalid token")
		return
	}
	if t.Purpose != models.TokenPurposeEmailChange {
		utils.JSONError(w, http.StatusBadRequest, "Invalid token purpose")
		return
	}
	uid := strconv.FormatUint(uint64(t.UserID), 10)
	user, err := h.Repo.GetUserByID(uid)
	if err != nil {
		utils.JSONError(w, http.StatusNotFound, "User not found")
		return
	}
	if time.Now().After(t.ExpiresAt) {
		// Expired: clear NewEmail and delete token
		if user.NewEmail != nil {
			_, _ = h.Repo.UpdateUser(uid, &models.User{NewEmail: nil})
		}
		_ = h.Tokens.DeleteByID(t.ID)
		utils.JSONError(w, http.StatusGone, "Email change token expired")
		return
	}

	if user.NewEmail == nil {
		utils.JSONError(w, http.StatusBadRequest, "No pending email change")
		return
	}
	// Apply new email
	newEmail := *user.NewEmail
	updates := &models.User{Email: newEmail, NewEmail: nil}
	if _, err := h.Repo.UpdateUser(uid, updates); err != nil {
		utils.JSONError(w, http.StatusInternalServerError, "Failed to update email")
		return
	}
	_ = h.Tokens.DeleteByID(t.ID)
	http.Redirect(w, r, clientBaseURL()+"/changeemail?status=ok", http.StatusSeeOther)
}
