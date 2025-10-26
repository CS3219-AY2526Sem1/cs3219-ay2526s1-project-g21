package routers

import (
	"net/http"
	"testing"

	"peerprep/user/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func TestAuthRoutesRegistered(t *testing.T) {
	r := chi.NewRouter()
	AuthRoutes(r, &handlers.AuthHandler{})

	expected := map[string]struct{}{
		"POST /api/v1/auth/login":    {},
		"POST /api/v1/auth/register": {},
		"GET /api/v1/auth/me":        {},
	}

	if err := chi.Walk(r, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := method + " " + route
		delete(expected, key)
		return nil
	}); err != nil {
		t.Fatalf("walk failed: %v", err)
	}

	if len(expected) != 0 {
		t.Fatalf("missing routes: %v", expected)
	}
}
