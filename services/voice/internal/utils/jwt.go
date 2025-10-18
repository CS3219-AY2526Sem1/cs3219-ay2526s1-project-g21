package utils

import (
	"errors"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret []byte

func init() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "your-secret-key" // Default for development
	}
	jwtSecret = []byte(secret)
}

// RoomTokenClaims represents the claims in a room access token
type RoomTokenClaims struct {
	MatchId string `json:"matchId"`
	UserId  string `json:"userId"`
	jwt.RegisteredClaims
}

// ValidateRoomToken validates a JWT token and returns the claims
func ValidateRoomToken(tokenString string) (*RoomTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RoomTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	return token.Claims.(*RoomTokenClaims), nil
}

// ExtractTokenFromHeader extracts the token from the Authorization header
func ExtractTokenFromHeader(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("authorization header missing")
	}

	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return "", errors.New("invalid authorization header format")
	}

	return authHeader[7:], nil
}
