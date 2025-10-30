package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestVerifyToken(t *testing.T) {
	secret := "test-secret"

	t.Run("missing header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		if _, err := VerifyToken(req, secret); err != ErrMissingAuthHeader {
			t.Fatalf("expected ErrMissingAuthHeader, got %v", err)
		}
	})

	t.Run("malformed header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Token abc")
		if _, err := VerifyToken(req, secret); err != ErrMissingAuthHeader {
			t.Fatalf("expected ErrMissingAuthHeader, got %v", err)
		}
	})

	t.Run("invalid signing method", func(t *testing.T) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatalf("failed to generate key: %v", err)
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub": "user",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		signed, err := token.SignedString(key)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+signed)
		if _, err := VerifyToken(req, secret); err != ErrInvalidToken {
			t.Fatalf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		signed, err := token.SignedString([]byte("other-secret"))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+signed)
		if _, err := VerifyToken(req, secret); err != ErrInvalidToken {
			t.Fatalf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("invalid claims type", func(t *testing.T) {
		orig := parseJWT
		defer func() { parseJWT = orig }()

		parseJWT = func(tokenStr string, keyFunc jwt.Keyfunc) (*jwt.Token, error) {
			token := jwt.New(jwt.SigningMethodHS256)
			token.Claims = &jwt.RegisteredClaims{}
			token.Valid = true
			if _, err := keyFunc(token); err != nil {
				return nil, err
			}
			return token, nil
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer fake")
		if _, err := VerifyToken(req, secret); err != ErrInvalidClaims {
			t.Fatalf("expected ErrInvalidClaims, got %v", err)
		}
	})

	t.Run("success", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": "user-123",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		signed, err := token.SignedString([]byte(secret))
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+signed)
		claims, err := VerifyToken(req, secret)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if claims["sub"] != "user-123" {
			t.Fatalf("expected sub 'user-123', got %v", claims["sub"])
		}
	})
}

func TestGetUserIDFromClaims(t *testing.T) {
	t.Run("string sub", func(t *testing.T) {
		id, err := GetUserIDFromClaims(jwt.MapClaims{"sub": "abc"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "abc" {
			t.Fatalf("expected id 'abc', got %q", id)
		}
	})

	t.Run("float64 sub", func(t *testing.T) {
		id, err := GetUserIDFromClaims(jwt.MapClaims{"sub": float64(42)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "42" {
			t.Fatalf("expected id '42', got %q", id)
		}
	})

	t.Run("missing sub", func(t *testing.T) {
		if _, err := GetUserIDFromClaims(jwt.MapClaims{}); err == nil {
			t.Fatalf("expected error for missing sub")
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		if _, err := GetUserIDFromClaims(jwt.MapClaims{"sub": true}); err == nil {
			t.Fatalf("expected error for invalid sub type")
		}
	})
}
