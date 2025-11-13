package utils

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// WriteJSON is an alias for JSON for compatibility
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	JSON(w, statusCode, data)
}
