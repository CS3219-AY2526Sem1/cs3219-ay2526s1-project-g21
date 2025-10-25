package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"peerprep/ai/internal/models"
	"peerprep/ai/internal/utils"
)

type AIHandler struct{}

func NewAIHandler() *AIHandler {
	return &AIHandler{}
}

func (h *AIHandler) ExplainHandler(w http.ResponseWriter, r *http.Request) {
	var req models.ExplainRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.JSON(w, http.StatusBadRequest, models.ErrorResponse{
			Code:    "invalid_json",
			Message: "Invalid JSON in request body",
		})
		return
	}

	// basic validation
	if req.Code == "" {
		utils.JSON(w, http.StatusBadRequest, models.ErrorResponse{
			Code:    "missing_code",
			Message: "Code field is required",
		})
		return
	}

	if req.Language == "" {
		utils.JSON(w, http.StatusBadRequest, models.ErrorResponse{
			Code:    "missing_language",
			Message: "Language field is required",
		})
		return
	}

	// validate supported languages
	supportedLanguages := map[string]bool{
		"python":     true,
		"java":       true,
		"cpp":        true,
		"javascript": true,
	}

	if !supportedLanguages[req.Language] {
		utils.JSON(w, http.StatusBadRequest, models.ErrorResponse{
			Code:    "unsupported_language",
			Message: "Language not supported. Supported languages: python, java, cpp, javascript",
		})
		return
	}

	// validate detail level
	if req.DetailLevel == "" {
		req.DetailLevel = "intermediate"
	}

	validDetailLevels := map[string]bool{
		"beginner":     true,
		"intermediate": true,
		"advanced":     true,
	}

	if !validDetailLevels[req.DetailLevel] {
		utils.JSON(w, http.StatusBadRequest, models.ErrorResponse{
			Code:    "invalid_detail_level",
			Message: "Detail level must be one of: beginner, intermediate, advanced",
		})
		return
	}

	// generate request ID if not provided
	if req.RequestID == "" {
		req.RequestID = generateRequestID()
	}

	// TODO: return actual AI-generated explanation
	response := models.ExplainResponse{
		Explanation: "This is a placeholder explanation for your " + req.Language + " code. " +
			"The AI explanation functionality will be implemented in the next phase. " +
			"Detail level: " + req.DetailLevel,
		RequestID: req.RequestID,
		Metadata: models.ExplanationMetadata{
			ProcessingTime: 150, // milliseconds
			DetailLevel:    req.DetailLevel,
		},
	}

	utils.JSON(w, http.StatusOK, response)
}

// generateRequestID creates a simple request ID (replace with proper UUID in production)
func generateRequestID() string {
	return "req_" + time.Now().Format("20060102150405")
}
