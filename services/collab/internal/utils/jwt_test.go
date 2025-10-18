package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateRoomTokenSuccess(t *testing.T) {
	prev := jwtSecret
	t.Cleanup(func() { jwtSecret = prev })
	jwtSecret = []byte("secret-key")

	tokenStr, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &RoomTokenClaims{
		MatchId: "match-1",
		UserId:  "user-1",
	}).SignedString(jwtSecret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	claims, err := ValidateRoomToken(tokenStr)
	if err != nil {
		t.Fatalf("expected valid token, got error %v", err)
	}
	if claims.MatchId != "match-1" || claims.UserId != "user-1" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestValidateRoomTokenInvalid(t *testing.T) {
	prev := jwtSecret
	t.Cleanup(func() { jwtSecret = prev })
	jwtSecret = []byte("secret-a")

	badToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &RoomTokenClaims{
		MatchId: "m",
		UserId:  "u",
	}).SignedString([]byte("other-secret"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	if _, err := ValidateRoomToken(badToken); err == nil {
		t.Fatalf("expected validation failure")
	}
}

func TestValidateRoomTokenUnexpectedMethod(t *testing.T) {
	prev := jwtSecret
	t.Cleanup(func() { jwtSecret = prev })
	jwSecret := []byte("secret-a")
	jwtSecret = jwSecret

	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tokenStr, err := jwt.NewWithClaims(jwt.SigningMethodRS256, &RoomTokenClaims{
		MatchId: "m",
		UserId:  "u",
	}).SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	if _, err := ValidateRoomToken(tokenStr); err == nil || !strings.Contains(err.Error(), "unexpected signing method") {
		t.Fatalf("expected signing method error, got %v", err)
	}
}

func TestValidateRoomTokenExpired(t *testing.T) {
	prev := jwtSecret
	t.Cleanup(func() { jwtSecret = prev })
	jwtSecret = []byte("secret-b")

	tokenStr, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &RoomTokenClaims{
		MatchId: "m",
		UserId:  "u",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}).SignedString(jwtSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	if _, err := ValidateRoomToken(tokenStr); err == nil {
		t.Fatalf("expected expiration error")
	}
}

func TestExtractTokenFromHeader(t *testing.T) {
	const token = "abc123"
	value, err := ExtractTokenFromHeader("Bearer " + token)
	if err != nil || value != token {
		t.Fatalf("unexpected result %q err=%v", value, err)
	}

	for _, header := range []string{"", "Token " + token, "Bearer"} {
		if _, err := ExtractTokenFromHeader(header); err == nil {
			t.Fatalf("expected error for header %q", header)
		}
	}
}

func TestLoggerMethods(t *testing.T) {
	logger := NewLogger()
	var buf strings.Builder
	logger.l.SetOutput(&buf)

	logger.Info("hi", "k", "v")
	logger.Warn("warn", "k2", "v2")
	logger.Error("err", "k3", "v3")

	output := buf.String()
	for _, needle := range []string{"INFO:", "WARN:", "ERROR:"} {
		if !strings.Contains(output, needle) {
			t.Fatalf("expected log output to contain %q; got %q", needle, output)
		}
	}
}
