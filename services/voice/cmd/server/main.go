package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"voice/internal/api"
)

func main() {
	// Start room cleanup routine
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			// This would be called on the room manager instance
			// For now, we'll just log
			log.Println("Running room cleanup...")
		}
	}()

	// Get port from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	// Create router
	router := api.NewRouter()

	// Start server
	addr := ":" + port
	log.Printf("Voice service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, router))
}
