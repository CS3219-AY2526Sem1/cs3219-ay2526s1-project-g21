package utils

import (
	"encoding/json"
	"math"
	"math/rand"
	"net/http"
	"time"

	"match/internal/models"

	"github.com/golang-jwt/jwt/v5"
)

// --- Helper Functions ---
func WriteJSON(w http.ResponseWriter, code int, resp models.Resp) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}

func EnableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// --- JWT Helper ---
func GenerateRoomToken(matchId, userId string, jwtSecret []byte) (string, error) {
	claims := jwt.MapClaims{
		"matchId": matchId,
		"userId":  userId,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func GetDifficultyToInt(diff string) int {
	switch diff {
	case models.DifficultyEasy:
		return 1
	case models.DifficultyMedium:
		return 2
	case models.DifficultyHard:
		return 3
	default:
		return 2
	}
}

func GetIntToDifficulty(val int) string {
	switch val {
	case 1:
		return models.DifficultyEasy
	case 2:
		return models.DifficultyMedium
	case 3:
		return models.DifficultyHard
	default:
		return models.DifficultyMedium
	}
}

func GetAverageDifficulty(diff1, diff2 string) string {
	d1 := GetDifficultyToInt(diff1)
	d2 := GetDifficultyToInt(diff2)
	avg := int(math.Floor(float64(d1+d2) / 2.0))
	return GetIntToDifficulty(avg)
}

func GetRandomCategory(cat1, cat2 string) string {
	if rand.Intn(2) == 0 {
		return cat1
	}
	return cat2
}
