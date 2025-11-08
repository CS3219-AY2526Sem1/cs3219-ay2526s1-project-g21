package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"peerprep/user/internal/models"
	"peerprep/user/internal/repositories"
	"peerprep/user/internal/testhelpers"
	"peerprep/user/internal/utils"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type mockUserRepo struct {
	createUserFn        func(*models.User) error
	getUserByUsernameFn func(string) (*models.User, error)
	getUserByEmailFn    func(string) (*models.User, error)
	getUserByIDFn       func(string) (*models.User, error)
	updateUserFn        func(string, *models.User) (*models.User, error)
	deleteUserFn        func(string) error
}

func (m *mockUserRepo) CreateUser(user *models.User) error {
	if m.createUserFn == nil {
		return nil
	}
	return m.createUserFn(user)
}

func (m *mockUserRepo) GetUserByUsername(username string) (*models.User, error) {
	if m.getUserByUsernameFn == nil {
		panic("unexpected call to GetUserByUsername")
	}
	return m.getUserByUsernameFn(username)
}

func (m *mockUserRepo) GetUserByEmail(email string) (*models.User, error) {
	if m.getUserByEmailFn == nil {
		panic("unexpected call to GetUserByEmail")
	}
	return m.getUserByEmailFn(email)
}

func (m *mockUserRepo) GetUserByID(id string) (*models.User, error) {
	if m.getUserByIDFn == nil {
		panic("unexpected call to GetUserByID")
	}
	return m.getUserByIDFn(id)
}

func (m *mockUserRepo) UpdateUser(id string, updates *models.User) (*models.User, error) {
	if m.updateUserFn == nil {
		panic("unexpected call to UpdateUser")
	}
	return m.updateUserFn(id, updates)
}

func (m *mockUserRepo) DeleteUser(id string) error {
	if m.deleteUserFn == nil {
		panic("unexpected call to DeleteUser")
	}
	return m.deleteUserFn(id)
}

type mockTokenRepo struct {
	createTokenFn               func(*models.Token) error
	getTokenByTokenFn           func(string) (*models.Token, error)
	getTokenByUserAndPurposeFn  func(uint, models.TokenPurpose) (*models.Token, error)
	deleteTokenByIDFn           func(uint) error
	deleteTokenByTokenFn        func(string) error
	deleteTokenByUserAndPurpose func(uint, models.TokenPurpose) error
	deleteExpiredFn             func(time.Time) (int64, error)
}

func (m *mockTokenRepo) Create(token *models.Token) error {
	if m.createTokenFn == nil {
		return nil
	}
	return m.createTokenFn(token)
}

func (m *mockTokenRepo) GetByToken(tokenStr string) (*models.Token, error) {
	if m.getTokenByTokenFn == nil {
		panic("unexpected call to GetByToken")
	}
	return m.getTokenByTokenFn(tokenStr)
}

func (m *mockTokenRepo) GetByUserAndPurpose(userID uint, purpose models.TokenPurpose) (*models.Token, error) {
	if m.getTokenByUserAndPurposeFn == nil {
		panic("unexpected call to GetByUserAndPurpose")
	}
	return m.getTokenByUserAndPurposeFn(userID, purpose)
}

func (m *mockTokenRepo) DeleteByID(id uint) error {
	if m.deleteTokenByIDFn == nil {
		panic("unexpected call to DeleteByID")
	}
	return m.deleteTokenByIDFn(id)
}

func (m *mockTokenRepo) DeleteByToken(tokenStr string) error {
	if m.deleteTokenByTokenFn == nil {
		panic("unexpected call to DeleteByToken")
	}
	return m.deleteTokenByTokenFn(tokenStr)
}

func (m *mockTokenRepo) DeleteByUserAndPurpose(userID uint, purpose models.TokenPurpose) error {
	if m.deleteTokenByUserAndPurpose == nil {
		panic("unexpected call to DeleteByUserAndPurpose")
	}
	return m.deleteTokenByUserAndPurpose(userID, purpose)
}

func (m *mockTokenRepo) DeleteExpired(before time.Time) (int64, error) {
	if m.deleteExpiredFn == nil {
		panic("unexpected call to DeleteExpired")
	}
	return m.deleteExpiredFn(before)
}

func newAuthHandlerWithDB(t *testing.T) (*AuthHandler, *repositories.UserRepository, *repositories.TokenRepository) {
	t.Helper()
	db := testhelpers.SetupTestDB(t)
	userRepo := &repositories.UserRepository{DB: db}
	tokenRepo := &repositories.TokenRepository{DB: db}
	return &AuthHandler{UserRepo: userRepo, TokenRepo: tokenRepo, JWTSecret: "test-secret"}, userRepo, tokenRepo
}

func makeToken(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func decodeResponse(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return out
}

func TestNewAuthHandler_SecretFromEnv(t *testing.T) {
	t.Setenv("JWT_SECRET", "custom")
	h := NewAuthHandler(&mockUserRepo{}, nil)
	if h.JWTSecret != "custom" {
		t.Fatalf("expected secret 'custom', got %q", h.JWTSecret)
	}
}

func TestNewAuthHandler_DefaultSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	h := NewAuthHandler(&mockUserRepo{}, nil)
	if h.JWTSecret != "dev" {
		t.Fatalf("expected default secret 'dev', got %q", h.JWTSecret)
	}
}

func TestAuthHandler_RegisterHandler(t *testing.T) {
	t.Run("invalid JSON payload", func(t *testing.T) {
		handler, _, _ := newAuthHandlerWithDB(t)
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("{invalid"))
		rec := httptest.NewRecorder()

		handler.RegisterHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("missing fields", func(t *testing.T) {
		handler, _, _ := newAuthHandlerWithDB(t)
		body := `{"username":"user"}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.RegisterHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("username repository error", func(t *testing.T) {
		repoErr := errors.New("db down")
		handler := &AuthHandler{
			UserRepo: &mockUserRepo{
				getUserByUsernameFn: func(string) (*models.User, error) { return nil, repoErr },
			},
			TokenRepo: &mockTokenRepo{},
		}

		body := `{"username":"user","email":"user@example.com","password":"Abcdefg!"}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.RegisterHandler(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Database error checking username") {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("username taken", func(t *testing.T) {
		existing := &models.User{Username: "user"}
		handler := &AuthHandler{
			UserRepo: &mockUserRepo{
				getUserByUsernameFn: func(string) (*models.User, error) { return existing, nil },
			},
			TokenRepo: &mockTokenRepo{},
		}
		body := `{"username":"user","email":"user@example.com","password":"Abcdefg!"}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.RegisterHandler(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("email repository error", func(t *testing.T) {
		handler := &AuthHandler{
			UserRepo: &mockUserRepo{
				getUserByUsernameFn: func(string) (*models.User, error) { return nil, repositories.ErrUserNotFound },
				getUserByEmailFn:    func(string) (*models.User, error) { return nil, errors.New("db down") },
			},
		}
		body := `{"username":"user","email":"user@example.com","password":"Abcdefg!"}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.RegisterHandler(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Database error checking email") {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("email taken", func(t *testing.T) {
		handler := &AuthHandler{
			UserRepo: &mockUserRepo{
				getUserByUsernameFn: func(string) (*models.User, error) { return nil, repositories.ErrUserNotFound },
				getUserByEmailFn:    func(string) (*models.User, error) { return &models.User{Email: "user@example.com"}, nil },
			},
		}
		body := `{"username":"user","email":"user@example.com","password":"Abcdefg!"}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.RegisterHandler(rec, req)

		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d", rec.Code)
		}
	})

	t.Run("invalid password", func(t *testing.T) {
		handler := &AuthHandler{
			UserRepo: &mockUserRepo{
				getUserByUsernameFn: func(string) (*models.User, error) { return nil, repositories.ErrUserNotFound },
				getUserByEmailFn:    func(string) (*models.User, error) { return nil, repositories.ErrUserNotFound },
			},
		}
		body := `{"username":"user","email":"user@example.com","password":"password"}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.RegisterHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("password hashing failure", func(t *testing.T) {
		handler := &AuthHandler{
			UserRepo: &mockUserRepo{
				getUserByUsernameFn: func(string) (*models.User, error) { return nil, repositories.ErrUserNotFound },
				getUserByEmailFn:    func(string) (*models.User, error) { return nil, repositories.ErrUserNotFound },
			},
		}
		orig := generatePasswordHash
		generatePasswordHash = func([]byte, int) ([]byte, error) { return nil, errors.New("hash failed") }
		defer func() { generatePasswordHash = orig }()

		body := `{"username":"user","email":"user@example.com","password":"Abcdefg!"}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.RegisterHandler(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Failed to hash password") {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("create user failure", func(t *testing.T) {
		handler := &AuthHandler{
			UserRepo: &mockUserRepo{
				getUserByUsernameFn: func(string) (*models.User, error) { return nil, repositories.ErrUserNotFound },
				getUserByEmailFn:    func(string) (*models.User, error) { return nil, repositories.ErrUserNotFound },
				createUserFn:        func(*models.User) error { return errors.New("insert failed") },
			},
		}
		body := `{"username":"user","email":"user@example.com","password":"Abcdefg!"}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.RegisterHandler(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Failed to create user") {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		handler, userRepo, _ := newAuthHandlerWithDB(t)
		body := `{"username":"user","email":"user@example.com","password":"Abcdefg!"}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.RegisterHandler(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
		resp := decodeResponse(t, rec)
		if resp["username"] != "user" {
			t.Fatalf("unexpected username in response: %v", resp["username"])
		}
		// Ensure persisted user stored hashed password
		user, err := userRepo.GetUserByUsername("user")
		if err != nil {
			t.Fatalf("failed to fetch created user: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("Abcdefg!")); err != nil {
			t.Fatalf("stored password is not a bcrypt hash")
		}
	})
}

func TestAuthHandler_LoginHandler(t *testing.T) {
	t.Run("invalid JSON payload", func(t *testing.T) {
		handler, _, _ := newAuthHandlerWithDB(t)
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("{invalid"))
		rec := httptest.NewRecorder()

		handler.LoginHandler(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("repository error treated as invalid credentials", func(t *testing.T) {
		handler := &AuthHandler{
			UserRepo: &mockUserRepo{
				getUserByUsernameFn: func(string) (*models.User, error) { return nil, errors.New("db error") },
			},
			TokenRepo: &mockTokenRepo{},
		}
		body := `{"username":"user","password":"secret"}`
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.LoginHandler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid credentials", func(t *testing.T) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.DefaultCost)
		handler := &AuthHandler{
			UserRepo: &mockUserRepo{
				getUserByUsernameFn: func(string) (*models.User, error) {
					return &models.User{Username: "user", PasswordHash: string(hash)}, nil
				},
			},
			TokenRepo: &mockTokenRepo{},
		}
		body := `{"username":"USER","password":"wrong"}`
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.LoginHandler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("token signing failure", func(t *testing.T) {
		hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
		handler := &AuthHandler{
			UserRepo: &mockUserRepo{
				getUserByUsernameFn: func(string) (*models.User, error) {
					return &models.User{Username: "user", PasswordHash: string(hash), Verified: true}, nil
				},
			},
			TokenRepo: &mockTokenRepo{},
			JWTSecret: "secret",
		}
		orig := signJWT
		signJWT = func(*jwt.Token, string) (string, error) { return "", errors.New("sign failed") }
		defer func() { signJWT = orig }()

		body := `{"username":"user","password":"password"}`
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.LoginHandler(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Failed to sign token") {
			t.Fatalf("unexpected body: %s", rec.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		handler, repo, _ := newAuthHandlerWithDB(t)
		password := "Abcdefg!"
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}
		user := &models.User{Username: "user", Email: "user@example.com", PasswordHash: string(hash), Verified: true}
		if err := repo.CreateUser(user); err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}

		body := `{"username":"USER","password":"Abcdefg!"}`
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		rec := httptest.NewRecorder()

		handler.LoginHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		resp := decodeResponse(t, rec)
		tokenStr, ok := resp["token"].(string)
		if !ok || tokenStr == "" {
			t.Fatalf("expected token in response, got %v", resp["token"])
		}
		reqCheck := httptest.NewRequest("GET", "/", nil)
		reqCheck.Header.Set("Authorization", "Bearer "+tokenStr)
		claims, err := utils.VerifyToken(reqCheck, handler.JWTSecret)
		if err != nil {
			t.Fatalf("VerifyToken failed: %v", err)
		}
		if claims["username"] != "user" {
			t.Fatalf("expected username claim 'user', got %v", claims["username"])
		}
	})
}

func TestAuthHandler_MeHandler(t *testing.T) {
	t.Run("missing token", func(t *testing.T) {
		handler, _, _ := newAuthHandlerWithDB(t)
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		rec := httptest.NewRecorder()

		handler.MeHandler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("invalid token subject", func(t *testing.T) {
		handler, _, _ := newAuthHandlerWithDB(t)
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{
			"sub": true,
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.MeHandler(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		handler := &AuthHandler{
			UserRepo: &mockUserRepo{
				getUserByIDFn: func(string) (*models.User, error) { return nil, repositories.ErrUserNotFound },
			},
			TokenRepo: &mockTokenRepo{},
			JWTSecret: "secret",
		}
		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{
			"sub": "1",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.MeHandler(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		handler, repo, _ := newAuthHandlerWithDB(t)
		user := &models.User{Username: "user", Email: "user@example.com", PasswordHash: "hash"}
		if err := repo.CreateUser(user); err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}

		token := makeToken(t, handler.JWTSecret, jwt.MapClaims{
			"sub": fmt.Sprintf("%d", user.ID),
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.MeHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		resp := decodeResponse(t, rec)
		if resp["email"] != "user@example.com" {
			t.Fatalf("unexpected response: %v", resp)
		}
	})
}
