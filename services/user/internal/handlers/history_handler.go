package handlers

import (
	"encoding/json"
	"net/http"

	"peerprep/user/internal/models"
	"peerprep/user/internal/repositories"

	"github.com/go-chi/chi/v5"
)

type HistoryHandler struct {
	Repo *repositories.HistoryRepository
}

// GetUserHistory retrieves all interview history for a user
func (h *HistoryHandler) GetUserHistory(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	if userID == "" {
		http.Error(w, "userId is required", http.StatusBadRequest)
		return
	}

	histories, err := h.Repo.GetByUserID(userID)
	if err != nil {
		http.Error(w, "Failed to retrieve history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(histories)
}

// GetSessionDetails retrieves details for a specific interview session
func (h *HistoryHandler) GetSessionDetails(w http.ResponseWriter, r *http.Request) {
	matchID := chi.URLParam(r, "matchId")
	if matchID == "" {
		http.Error(w, "matchId is required", http.StatusBadRequest)
		return
	}

	history, err := h.Repo.GetByMatchID(matchID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// CreateHistory creates a new interview history record (internal use via Redis events)
func (h *HistoryHandler) CreateHistory(history *models.InterviewHistory) error {
	return h.Repo.Create(history)
}
