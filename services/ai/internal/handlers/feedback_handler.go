package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"peerprep/ai/internal/feedback"
	"peerprep/ai/internal/models"
	"peerprep/ai/internal/utils"

	"github.com/go-chi/chi/v5"
)

type FeedbackHandler struct {
	feedbackManager *feedback.FeedbackManager
}

func NewFeedbackHandler(feedbackManager *feedback.FeedbackManager) *FeedbackHandler {
	return &FeedbackHandler{
		feedbackManager: feedbackManager,
	}
}

// SubmitFeedbackRequest represents the request body for feedback submission
type SubmitFeedbackRequest struct {
	IsPositive bool `json:"is_positive"`
}

// SubmitFeedback handles POST /api/v1/ai/feedback/:request_id
func (fh *FeedbackHandler) SubmitFeedback(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "request_id")
	if requestID == "" {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{
			OK:   false,
			Info: "request_id is required",
		})
		return
	}

	var req SubmitFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{
			OK:   false,
			Info: "invalid request body",
		})
		return
	}

	// Store feedback
	if err := fh.feedbackManager.SubmitFeedback(requestID, req.IsPositive); err != nil {
		log.Printf("Failed to submit feedback: %v", err)
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{
			OK:   false,
			Info: "failed to submit feedback: " + err.Error(),
		})
		return
	}

	utils.WriteJSON(w, http.StatusOK, models.Resp{
		OK:   true,
		Info: "feedback submitted successfully",
	})
}

// ExportFeedback handles GET /api/v1/ai/feedback/export
// Query params:
// - days: number of days to look back (default: 7)
// - limit: maximum number of records (optional)
// - format: "jsonl" (default) or "json"
func (fh *FeedbackHandler) ExportFeedback(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	daysParam := r.URL.Query().Get("days")
	days := 7 // default
	if daysParam != "" {
		if d, err := strconv.Atoi(daysParam); err == nil && d > 0 {
			days = d
		}
	}

	limitParam := r.URL.Query().Get("limit")
	limit := 0 // no limit by default
	if limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 {
			limit = l
		}
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "jsonl"
	}

	// Get feedback since N days ago
	since := time.Now().AddDate(0, 0, -days)
	feedback, err := fh.feedbackManager.GetFeedbackSince(since, limit)
	if err != nil {
		log.Printf("Failed to get feedback: %v", err)
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{
			OK:   false,
			Info: "failed to export feedback",
		})
		return
	}

	if len(feedback) == 0 {
		utils.WriteJSON(w, http.StatusOK, models.Resp{
			OK:   true,
			Info: "no feedback to export",
		})
		return
	}

	// Export based on format
	if format == "jsonl" {
		jsonlData, err := fh.feedbackManager.ExportToJSONL(feedback)
		if err != nil {
			log.Printf("Failed to export to JSONL: %v", err)
			utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{
				OK:   false,
				Info: "failed to export to JSONL",
			})
			return
		}

		// Return JSONL data
		w.Header().Set("Content-Type", "application/jsonl")
		w.Header().Set("Content-Disposition", "attachment; filename=feedback_export.jsonl")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonlData)
	} else {
		// Return as JSON
		utils.WriteJSON(w, http.StatusOK, models.Resp{
			OK:   true,
			Info: feedback,
		})
	}

	log.Printf("Exported %d feedback records (last %d days)", len(feedback), days)
}

// GetFeedbackStats handles GET /api/v1/ai/feedback/stats
func (fh *FeedbackHandler) GetFeedbackStats(w http.ResponseWriter, r *http.Request) {
	stats, err := fh.feedbackManager.GetFeedbackStats()
	if err != nil {
		log.Printf("Failed to get feedback stats: %v", err)
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{
			OK:   false,
			Info: "failed to get feedback stats",
		})
		return
	}

	utils.WriteJSON(w, http.StatusOK, models.Resp{
		OK:   true,
		Info: stats,
	})
}
