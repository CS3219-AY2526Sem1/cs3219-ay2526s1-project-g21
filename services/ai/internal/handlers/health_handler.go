package handlers

import (
	"net/http"
	"peerprep/ai/internal/utils"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (handler *HealthHandler) HealthzHandler(writer http.ResponseWriter, request *http.Request) {
	utils.JSON(writer, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "ai",
		"version": "1.0.0",
	})
}

func (handler *HealthHandler) ReadyzHandler(writer http.ResponseWriter, request *http.Request) {
	utils.JSON(writer, http.StatusOK, map[string]string{
		"status":  "ready",
		"service": "ai",
	})
}
