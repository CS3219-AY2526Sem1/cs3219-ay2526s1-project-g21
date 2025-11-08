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
	promptManager prompts.PromptProvider
	logger        *zap.Logger
}

func NewAIHandler(provider llm.Provider, promptManager prompts.PromptProvider, logger *zap.Logger) *AIHandler {
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
	req.RequestID = ensureRequestID(req.RequestID)

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

// ensureRequestID generates a request ID if one is not provided
func ensureRequestID(requestID string) string {
	if requestID == "" {
		return generateRequestID()
	}
	return requestID
}

func (h *AIHandler) HintHandler(w http.ResponseWriter, r *http.Request) {
	// Get the validated request from middleware
	req := middleware.GetValidatedRequest[*models.HintRequest](r)

	req.RequestID = ensureRequestID(req.RequestID)

	// Build prompt directly from hint.yaml
	promptData := map[string]interface{}{
		"Language":  req.Language,
		"Code":      req.Code,
		"Question":  req.Question,
		"HintLevel": req.HintLevel,
	}

	prompt, err := h.promptManager.BuildPrompt("hint", "default", promptData)
	if err != nil {
		h.logger.Error("Failed to build prompt", zap.Error(err), zap.String("request_id", req.RequestID))
		utils.JSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "prompt_error",
			Message: "Failed to build AI prompt",
		})
		return
	}

	// Reuse same provider call as explain
	result, err := h.provider.GenerateExplanation(r.Context(), prompt, req.RequestID, req.HintLevel)
	if err != nil {
		h.logger.Error("AI provider error", zap.Error(err), zap.String("request_id", req.RequestID))
		utils.JSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "ai_error",
			Message: "Failed to generate hint",
		})
		return
	}

	resp := models.HintResponse{
		Hint:      result.Explanation,
		RequestID: req.RequestID,
		Metadata:  result.Metadata,
	}

	utils.JSON(w, http.StatusOK, resp)
}

func (h *AIHandler) TestsHandler(w http.ResponseWriter, r *http.Request) {
	req := middleware.GetValidatedRequest[*models.TestGenRequest](r)
	req.RequestID = ensureRequestID(req.RequestID)

	// Build prompt from templates/tests.yaml
	promptData := map[string]interface{}{
		"Language":  req.Language,
		"Code":      req.Code,
		"Question":  req.Question,
		"Framework": req.Framework,
	}
	prompt, err := h.promptManager.BuildPrompt("tests", "default", promptData)
	if err != nil {
		h.logger.Error("Failed to build prompt", zap.Error(err), zap.String("request_id", req.RequestID))
		utils.JSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "prompt_error",
			Message: "Failed to build AI prompt",
		})
		return
	}

	// Reuse provider call
	out, err := h.provider.GenerateExplanation(r.Context(), prompt, req.RequestID, models.DefaultDetailLevel)
	if err != nil {
		h.logger.Error("AI provider error", zap.Error(err), zap.String("request_id", req.RequestID))
		utils.JSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "ai_error",
			Message: "Failed to generate test cases",
		})
		return
	}

	resp := models.TestGenResponse{
		TestsCode: out.Explanation,
		RequestID: req.RequestID,
		Metadata:  out.Metadata,
	}
	utils.JSON(w, http.StatusOK, resp)
}

func (h *AIHandler) RefactorTipsHandler(w http.ResponseWriter, r *http.Request) {
	req := middleware.GetValidatedRequest[*models.RefactorTipsRequest](r)
	req.RequestID = ensureRequestID(req.RequestID)

	data := map[string]interface{}{
		"Language": req.Language,
		"Code":     utils.AddLineNumbers(req.Code),
		"Question": req.Question,
	}

	prompt, err := h.promptManager.BuildPrompt("refactor_tips", "default", data)
	if err != nil {
		h.logger.Error("Failed to build prompt", zap.Error(err), zap.String("request_id", req.RequestID))
		utils.JSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "prompt_error",
			Message: "Failed to build prompt",
		})
		return
	}

	result, err := h.provider.GenerateExplanation(r.Context(), prompt, req.RequestID, models.DefaultDetailLevel)
	if err != nil {
		h.logger.Error("AI provider error", zap.Error(err), zap.String("request_id", req.RequestID))
		utils.JSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Code:    "ai_error",
			Message: "Failed to generate refactor tips",
		})
		return
	}

	cleaned := utils.StripFences(result.Explanation)

	utils.JSON(w, http.StatusOK, models.RefactorTipsTextResponse{
		TipsText:  cleaned,
		RequestID: req.RequestID,
		Metadata:  result.Metadata,
	})
}
