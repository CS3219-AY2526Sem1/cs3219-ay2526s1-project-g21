package main

import (
	"log"
	"net/http"
	"os"

	"voice/internal/api"
)

func main() {

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
