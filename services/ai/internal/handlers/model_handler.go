package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"peerprep/ai/internal/models"
	"peerprep/ai/internal/tuning"
	"peerprep/ai/internal/utils"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type ModelHandler struct {
	db          *gorm.DB
	geminiTuner *tuning.GeminiTuner
}

func NewModelHandler(db *gorm.DB, geminiTuner *tuning.GeminiTuner) *ModelHandler {
	return &ModelHandler{
		db:          db,
		geminiTuner: geminiTuner,
	}
}

// UpdateTrafficWeightRequest represents the request body for updating traffic weight
type UpdateTrafficWeightRequest struct {
	TrafficWeight int `json:"traffic_weight"`
}

// UpdateTrafficWeight handles PUT /api/v1/ai/models/:model_id/traffic
// Updates the traffic weight for a specific model version
func (mh *ModelHandler) UpdateTrafficWeight(w http.ResponseWriter, r *http.Request) {
	modelIDStr := chi.URLParam(r, "model_id")
	if modelIDStr == "" {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{
			OK:   false,
			Info: "model_id is required",
		})
		return
	}

	modelID, err := strconv.ParseUint(modelIDStr, 10, 32)
	if err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{
			OK:   false,
			Info: "invalid model_id",
		})
		return
	}

	var req UpdateTrafficWeightRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{
			OK:   false,
			Info: "invalid request body",
		})
		return
	}

	// Validate traffic weight (0-100)
	if req.TrafficWeight < 0 || req.TrafficWeight > 100 {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{
			OK:   false,
			Info: "traffic_weight must be between 0 and 100",
		})
		return
	}

	// Update traffic weight using tuner
	if err := mh.geminiTuner.ActivateModel(uint(modelID), req.TrafficWeight); err != nil {
		log.Printf("Failed to update traffic weight: %v", err)
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{
			OK:   false,
			Info: "failed to update traffic weight: " + err.Error(),
		})
		return
	}

	utils.WriteJSON(w, http.StatusOK, models.Resp{
		OK:   true,
		Info: "traffic weight updated successfully",
	})
}

// DeactivateModel handles PUT /api/v1/ai/models/:model_id/deactivate
// Deactivates a specific model version (sets is_active=false, traffic_weight=0)
func (mh *ModelHandler) DeactivateModel(w http.ResponseWriter, r *http.Request) {
	modelIDStr := chi.URLParam(r, "model_id")
	if modelIDStr == "" {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{
			OK:   false,
			Info: "model_id is required",
		})
		return
	}

	modelID, err := strconv.ParseUint(modelIDStr, 10, 32)
	if err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{
			OK:   false,
			Info: "invalid model_id",
		})
		return
	}

	// Deactivate model
	if err := mh.geminiTuner.DeactivateModel(uint(modelID)); err != nil {
		log.Printf("Failed to deactivate model: %v", err)
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{
			OK:   false,
			Info: "failed to deactivate model: " + err.Error(),
		})
		return
	}

	utils.WriteJSON(w, http.StatusOK, models.Resp{
		OK:   true,
		Info: "model deactivated successfully",
	})
}

// ListModels handles GET /api/v1/ai/models
// Lists all model versions with their status and traffic weights
func (mh *ModelHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	var modelVersions []models.ModelVersion

	query := mh.db.Order("created_at DESC")

	// Optional filter: only active models
	if r.URL.Query().Get("active") == "true" {
		query = query.Where("is_active = ?", true)
	}

	if err := query.Find(&modelVersions).Error; err != nil {
		log.Printf("Failed to list models: %v", err)
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{
			OK:   false,
			Info: "failed to list models",
		})
		return
	}

	utils.WriteJSON(w, http.StatusOK, models.Resp{
		OK:   true,
		Info: modelVersions,
	})
}

// GetModelStats handles GET /api/v1/ai/models/:model_id/stats
// Returns performance statistics for a specific model version
func (mh *ModelHandler) GetModelStats(w http.ResponseWriter, r *http.Request) {
	modelIDStr := chi.URLParam(r, "model_id")
	if modelIDStr == "" {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{
			OK:   false,
			Info: "model_id is required",
		})
		return
	}

	modelID, err := strconv.ParseUint(modelIDStr, 10, 32)
	if err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, models.Resp{
			OK:   false,
			Info: "invalid model_id",
		})
		return
	}

	// Get model version
	var modelVersion models.ModelVersion
	if err := mh.db.First(&modelVersion, modelID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.WriteJSON(w, http.StatusNotFound, models.Resp{
				OK:   false,
				Info: "model not found",
			})
			return
		}
		log.Printf("Failed to get model: %v", err)
		utils.WriteJSON(w, http.StatusInternalServerError, models.Resp{
			OK:   false,
			Info: "failed to get model",
		})
		return
	}

	// Get feedback statistics for this model version
	type FeedbackStats struct {
		TotalFeedback    int64   `json:"total_feedback"`
		PositiveFeedback int64   `json:"positive_feedback"`
		NegativeFeedback int64   `json:"negative_feedback"`
		PositiveRate     float64 `json:"positive_rate"`
	}

	var stats FeedbackStats
	mh.db.Model(&models.AIFeedback{}).
		Where("model_version = ?", modelVersion.VersionName).
		Count(&stats.TotalFeedback)

	mh.db.Model(&models.AIFeedback{}).
		Where("model_version = ? AND is_positive = ?", modelVersion.VersionName, true).
		Count(&stats.PositiveFeedback)

	stats.NegativeFeedback = stats.TotalFeedback - stats.PositiveFeedback
	if stats.TotalFeedback > 0 {
		stats.PositiveRate = float64(stats.PositiveFeedback) / float64(stats.TotalFeedback) * 100
	}

	response := map[string]interface{}{
		"model":    modelVersion,
		"feedback": stats,
	}

	utils.WriteJSON(w, http.StatusOK, models.Resp{
		OK:   true,
		Info: response,
	})
}
