package routers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	matchManager "match/internal/match_management"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestMatchRoutes(t *testing.T) {
	secret := []byte("test-secret")
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379", // This won't connect in test, but that's okay for route testing
	})
	mm := matchManager.NewMatchManager(secret, rdb)

	r := chi.NewRouter()
	MatchRoutes(r, mm)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "Join endpoint exists",
			method:         http.MethodPost,
			path:           "/api/v1/match/join",
			expectedStatus: http.StatusBadRequest, // Will fail validation, but route exists
		},
		{
			name:           "Cancel endpoint exists",
			method:         http.MethodPost,
			path:           "/api/v1/match/cancel",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Check endpoint exists",
			method:         http.MethodGet,
			path:           "/api/v1/match/check",
			expectedStatus: http.StatusBadRequest, // Missing userId
		},
		{
			name:           "Done endpoint exists",
			method:         http.MethodPost,
			path:           "/api/v1/match/done",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Handshake endpoint exists",
			method:         http.MethodPost,
			path:           "/api/v1/match/handshake",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "WebSocket endpoint exists",
			method:         http.MethodGet,
			path:           "/api/v1/match/ws",
			expectedStatus: http.StatusBadRequest, // Will fail upgrade, but route exists
		},
		{
			name:           "Non-existent endpoint returns 404",
			method:         http.MethodGet,
			path:           "/api/v1/match/nonexistent",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Route %s %s should return status %d", tt.method, tt.path, tt.expectedStatus)
		})
	}
}

func TestMatchRoutes_Options(t *testing.T) {
	secret := []byte("test-secret")
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	mm := matchManager.NewMatchManager(secret, rdb)

	r := chi.NewRouter()
	MatchRoutes(r, mm)

	// Test OPTIONS requests
	tests := []struct {
		name           string
		path           string
		expectedStatus int
	}{
		{
			name:           "OPTIONS on join",
			path:           "/api/v1/match/join",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "OPTIONS on cancel",
			path:           "/api/v1/match/cancel",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "OPTIONS on check",
			path:           "/api/v1/match/check",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "OPTIONS on done",
			path:           "/api/v1/match/done",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "OPTIONS on handshake",
			path:           "/api/v1/match/handshake",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "OPTIONS %s should return status %d", tt.path, tt.expectedStatus)
		})
	}
}

func TestMatchRoutes_RouteStructure(t *testing.T) {
	secret := []byte("test-secret")
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	mm := matchManager.NewMatchManager(secret, rdb)

	r := chi.NewRouter()
	MatchRoutes(r, mm)

	// Verify all routes are under /api/v1/match prefix
	paths := []string{
		"/api/v1/match/join",
		"/api/v1/match/cancel",
		"/api/v1/match/check",
		"/api/v1/match/done",
		"/api/v1/match/handshake",
		"/api/v1/match/ws",
	}

	for _, path := range paths {
		t.Run("Route under prefix: "+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Should not be 404 if route is properly registered
			assert.NotEqual(t, http.StatusNotFound, w.Code, "Route %s should be registered", path)
		})
	}
}

func TestMatchRoutes_MethodRouting(t *testing.T) {
	secret := []byte("test-secret")
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	mm := matchManager.NewMatchManager(secret, rdb)

	r := chi.NewRouter()
	MatchRoutes(r, mm)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "POST to /join",
			method:         http.MethodPost,
			path:           "/api/v1/match/join",
			expectedStatus: http.StatusBadRequest, // Missing body
		},
		{
			name:           "GET to /join (should fail)",
			method:         http.MethodGet,
			path:           "/api/v1/match/join",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "POST to /cancel",
			method:         http.MethodPost,
			path:           "/api/v1/match/cancel",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GET to /check",
			method:         http.MethodGet,
			path:           "/api/v1/match/check",
			expectedStatus: http.StatusBadRequest, // Missing userId
		},
		{
			name:           "POST to /check (should fail)",
			method:         http.MethodPost,
			path:           "/api/v1/match/check",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "POST to /done",
			method:         http.MethodPost,
			path:           "/api/v1/match/done",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "POST to /handshake",
			method:         http.MethodPost,
			path:           "/api/v1/match/handshake",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "GET to /ws",
			method:         http.MethodGet,
			path:           "/api/v1/match/ws",
			expectedStatus: http.StatusBadRequest, // Missing userId
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, "Method %s on %s should return status %d", tt.method, tt.path, tt.expectedStatus)
		})
	}
}

func TestMatchRoutes_CORSPreflight(t *testing.T) {
	secret := []byte("test-secret")
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	mm := matchManager.NewMatchManager(secret, rdb)

	r := chi.NewRouter()
	MatchRoutes(r, mm)

	// Test that OPTIONS requests work on all endpoints
	endpoints := []string{
		"/api/v1/match/join",
		"/api/v1/match/cancel",
		"/api/v1/match/check",
		"/api/v1/match/done",
		"/api/v1/match/handshake",
	}

	for _, endpoint := range endpoints {
		t.Run("OPTIONS on "+endpoint, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, endpoint, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// All endpoints should handle OPTIONS
			assert.Equal(t, http.StatusOK, w.Code, "OPTIONS on %s should return 200", endpoint)
		})
	}
}

func TestMatchRoutes_InvalidPaths(t *testing.T) {
	secret := []byte("test-secret")
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	mm := matchManager.NewMatchManager(secret, rdb)

	r := chi.NewRouter()
	MatchRoutes(r, mm)

	invalidPaths := []string{
		"/api/v1/match",
		"/api/v1/match/",
		"/api/v1/match/invalid",
		"/api/v1/match/join/invalid",
		"/api/v1/other",
		"/match/join",
	}

	for _, path := range invalidPaths {
		t.Run("Invalid path: "+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Should return 404 for invalid paths
			assert.Equal(t, http.StatusNotFound, w.Code, "Invalid path %s should return 404", path)
		})
	}
}

func TestMatchRoutes_PathPrefix(t *testing.T) {
	secret := []byte("test-secret")
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	mm := matchManager.NewMatchManager(secret, rdb)

	r := chi.NewRouter()
	MatchRoutes(r, mm)

	// Verify that routes outside /api/v1/match prefix don't work
	outsidePaths := []string{
		"/join",
		"/cancel",
		"/check",
		"/match/join",
		"/v1/match/join",
	}

	for _, path := range outsidePaths {
		t.Run("Outside prefix: "+path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			// Should return 404 for paths outside the prefix
			assert.Equal(t, http.StatusNotFound, w.Code, "Path %s outside prefix should return 404", path)
		})
	}
}
