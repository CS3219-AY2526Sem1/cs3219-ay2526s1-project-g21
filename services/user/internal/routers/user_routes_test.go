package routers

import (
	"net/http"
	"testing"

	"peerprep/user/internal/handlers"

	"github.com/go-chi/chi/v5"
)

func TestUserRoutesRegistered(t *testing.T) {
	r := chi.NewRouter()
	UserRoutes(r, &handlers.UserHandler{})

	expected := map[string]struct{}{
		"PUT /api/v1/users/{id}":    {},
		"DELETE /api/v1/users/{id}": {},
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
