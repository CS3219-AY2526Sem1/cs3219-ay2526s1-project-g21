package handlers

import (
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
	// Get the validated request from middleware
	req := r.Context().Value("validated_request").(*models.ExplainRequest)

	// Generate request ID if not provided
	if req.RequestID == "" {
		req.RequestID = generateRequestID()
	}

	// TODO: Replace with actual AI service call
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

// generateRequestID creates a simple request ID 
// TODO: replace with proper UUID or remove completely
func generateRequestID() string {
	return "req_" + time.Now().Format("20060102150405")
}
