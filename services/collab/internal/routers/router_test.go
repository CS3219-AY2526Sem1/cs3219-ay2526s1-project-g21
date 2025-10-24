package routers

import (
	"log"
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
	log.Println(server.URL)
	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
