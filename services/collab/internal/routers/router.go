package routers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"collab/internal/api"
	"collab/internal/utils"
)

func New(log *utils.Logger) http.Handler {
	h := api.NewHandlers(log)
	r := chi.NewRouter()

	r.Get("/api/v1/healthz", h.Health)

	r.Get("/api/v1/languages", h.ListLanguages)
	r.Post("/api/v1/format", h.FormatCode)

	r.Post("/api/v1/run", h.RunOnce)

	r.Get("/ws/session/{id}", h.CollabWS)

	return r
}
