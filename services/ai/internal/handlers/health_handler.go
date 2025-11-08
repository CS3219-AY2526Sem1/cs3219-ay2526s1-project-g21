package handlers

import (
	"net/http"
	"peerprep/ai/internal/config"
	"peerprep/ai/internal/llm"
	"peerprep/ai/internal/prompts"
	"peerprep/ai/internal/utils"
)

type ReadinessCheck struct {
	Status  string `json:"status"` // "ok" | "failed"
	Message string `json:"message,omitempty"`
}

type ReadinessResponse struct {
	Status  string                    `json:"status"`  // "ready" | "not_ready"
	Service string                    `json:"service"` // Service name
	Checks  map[string]ReadinessCheck `json:"checks"`  // Individual check results
}

type HealthHandler struct {
	provider      llm.Provider
	promptManager prompts.PromptProvider
	config        *config.Config
}

func NewHealthHandler(provider llm.Provider, promptManager prompts.PromptProvider, cfg *config.Config) *HealthHandler {
	return &HealthHandler{
		provider:      provider,
		promptManager: promptManager,
		config:        cfg,
	}
}

func (handler *HealthHandler) HealthzHandler(writer http.ResponseWriter, request *http.Request) {
	utils.JSON(writer, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "ai",
		"version": "1.0.0",
	})
}

func (handler *HealthHandler) ReadyzHandler(writer http.ResponseWriter, request *http.Request) {
	checks := make(map[string]ReadinessCheck)
	allChecksPass := true

	// verify AI provider is initialized
	if handler.provider == nil {
		checks["provider"] = ReadinessCheck{
			Status:  "failed",
			Message: "AI provider not initialized",
		}
		allChecksPass = false
	} else {
		checks["provider"] = ReadinessCheck{
			Status: "ok",
		}
	}

	// verify prompt manager has templates loaded
	if handler.promptManager == nil {
		checks["prompt_manager"] = ReadinessCheck{
			Status:  "failed",
			Message: "Prompt manager not initialized",
		}
		allChecksPass = false
	} else {
		templates := handler.promptManager.GetTemplates()
		if len(templates) == 0 {
			checks["prompt_manager"] = ReadinessCheck{
				Status:  "failed",
				Message: "No prompt templates loaded",
			}
			allChecksPass = false
		} else {
			checks["prompt_manager"] = ReadinessCheck{
				Status: "ok",
			}
		}
	}

	// verify configuration is valid
	if handler.config == nil {
		checks["configuration"] = ReadinessCheck{
			Status:  "failed",
			Message: "Configuration not loaded",
		}
		allChecksPass = false
	} else {
		checks["configuration"] = ReadinessCheck{
			Status: "ok",
		}
	}

	response := ReadinessResponse{
		Service: "ai",
		Checks:  checks,
	}

	if allChecksPass {
		response.Status = "ready"
		utils.JSON(writer, http.StatusOK, response)
	} else {
		response.Status = "not_ready"
		utils.JSON(writer, http.StatusServiceUnavailable, response)
	}
}
