package routers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"collab/internal/api"
	"collab/internal/room_management"
	"collab/internal/utils"
)

func New(log *utils.Logger, roomManager *room_management.RoomManager) http.Handler {
	h := api.NewHandlers(log, roomManager)
	r := chi.NewRouter()

	r.Get("/api/v1/healthz", h.Health)

	r.Get("/api/v1/languages", h.ListLanguages)
	r.Post("/api/v1/format", h.FormatCode)

	r.Post("/api/v1/run", h.RunOnce)

	// Room status endpoint
	r.Get("/api/v1/room/{matchId}", h.GetRoomStatus)
	r.Post("/api/v1/room/{matchId}/reroll", h.RerollQuestion)
	r.Get("/api/v1/room/active/{userId}", h.GetActiveRoom)

	r.Get("/ws/session/{id}", h.CollabWS)

	return r
}
