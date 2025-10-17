package routers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"collab/internal/room_management"
	"collab/internal/utils"
)

func TestNewRouterHealthEndpoint(t *testing.T) {
	logger := utils.NewLogger()
	manager := room_management.NewRoomManager("localhost:0", "http://localhost")

	handler := New(logger, manager)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/api/v1/healthz")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
