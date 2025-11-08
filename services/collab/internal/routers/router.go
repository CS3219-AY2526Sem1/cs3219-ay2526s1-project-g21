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

	r.Get("/healthz", h.Health)

	r.Get("/languages", h.ListLanguages)
	r.Post("/format", h.FormatCode)

	r.Post("/run", h.RunOnce)

	// Room status endpoint
	r.Get("/room/{matchId}", h.GetRoomStatus)
	r.Post("/room/{matchId}/reroll", h.RerollQuestion)
	r.Get("/room/active/{userId}", h.GetActiveRoom)

	r.Get("/room/{matchId}/getInstanceID", h.GetInstanceID)

	r.Get("/ws/session/{id}", h.CollabWS)

	return r
}
