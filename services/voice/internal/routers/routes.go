package routers

import (
	"net/http"

	"voice/internal/handlers"
	"voice/internal/utils"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(log *utils.Logger, redisAddr string) http.Handler {
	h := handlers.NewHandlers(redisAddr)
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60))

	// CORS middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// Health check
	r.Get("/health", h.Health)

	r.Route("/api/v1", func(r chi.Router) {
		// WebRTC configuration
		r.Get("/webrtc/config", h.GetWebRTCConfig)

		// WebSocket for voice chat
		r.Get("/room/{roomId}/voice", h.VoiceChatWS)
	})

	return r
}
