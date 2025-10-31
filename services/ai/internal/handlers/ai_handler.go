package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"peerprep/ai/internal/llm"
	"peerprep/ai/internal/middleware"
	"peerprep/ai/internal/models"
	"peerprep/ai/internal/prompts"
	"peerprep/ai/internal/utils"
)

type AIHandler struct {
	provider      llm.Provider
	promptManager *prompts.PromptManager
	logger        *zap.Logger
}

func NewAIHandler(provider llm.Provider, promptManager *prompts.PromptManager, logger *zap.Logger) *AIHandler {
	return &AIHandler{
		provider:      provider,
		promptManager: promptManager,
		logger:        logger,
	}
}

func (h *AIHandler) ExplainHandler(w http.ResponseWriter, r *http.Request) {
	// Get the validated request from middleware
	req := middleware.GetValidatedRequest[*models.ExplainRequest](r)

	// Generate request ID if not provided
	if req.RequestID == "" {
		req.RequestID = generateRequestID()
	}

	// build the prompt using the prompt manager
	promptData := map[string]interface{}{
		"Language": req.Language,
		"Code":     req.Code,
	}

	prompt, err := h.promptManager.BuildPrompt("explain", req.DetailLevel, promptData)
	if err != nil {
		h.logger.Error("Failed to build prompt", zap.Error(err), zap.String("request_id", req.RequestID))
		utils.JSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "prompt_error",
			Message: "Failed to build AI prompt",
		})
		return
	}

	// call the AI provider with the built prompt
	response, err := h.provider.GenerateExplanation(r.Context(), prompt, req.RequestID, req.DetailLevel)
	if err != nil {
		h.logger.Error("AI provider error", zap.Error(err), zap.String("request_id", req.RequestID))
		utils.JSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "ai_error",
			Message: "Failed to generate explanation",
		})
		return
	}

	h.logger.Info("Explanation generated successfully",
		zap.String("request_id", req.RequestID),
		zap.String("provider", h.provider.GetProviderName()),
		zap.Int("processing_time_ms", response.Metadata.ProcessingTime))

	utils.JSON(w, http.StatusOK, response)
}

func generateRequestID() string {
	return uuid.New().String()
}
