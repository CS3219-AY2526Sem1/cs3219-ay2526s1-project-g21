package utils

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

var parseJWT = func(tokenStr string, keyFunc jwt.Keyfunc) (*jwt.Token, error) {
	return jwt.Parse(tokenStr, keyFunc)
}

var (
	ErrMissingAuthHeader = errors.New("missing or malformed Authorization header")
	ErrInvalidToken      = errors.New("invalid token")
	ErrInvalidClaims     = errors.New("invalid token claims")
)

// VerifyToken fetches the Authorization header, validates the JWT,
// and returns the claims if everything is valid.
func VerifyToken(r *http.Request, secret string) (jwt.MapClaims, error) {
	authz := r.Header.Get("Authorization")
	if authz == "" || !strings.HasPrefix(authz, "Bearer ") {
		return nil, ErrMissingAuthHeader
	}
	tokenStr := strings.TrimPrefix(authz, "Bearer ")

	token, err := parseJWT(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenUnverifiable
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidClaims
	}
	return claims, nil
}

// GetUserIDFromClaims extracts the "sub" (user ID) from claims safely as a string.
func GetUserIDFromClaims(claims jwt.MapClaims) (string, error) {
	sub, ok := claims["sub"]
	if !ok {
		return "", errors.New("missing sub claim")
	}

	switch v := sub.(type) {
	case string:
		return v, nil
	case float64:
		// JWT numbers get decoded as float64
		return fmt.Sprintf("%d", int64(v)), nil
	default:
		return "", errors.New("invalid sub claim type")
	}
}
