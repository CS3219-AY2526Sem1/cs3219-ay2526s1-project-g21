package main

import (
	"log"
	"net/http"
	"os"

	"voice/internal/routers"
	"voice/internal/utils"
)

var (
	listenAndServe   = http.ListenAndServe
	defaultRedisAddr = "redis:6379"
	defaultPort      = "8085"
)

func main() {
	logger := utils.NewLogger()

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = defaultRedisAddr
	}

	// Create router
	router := routers.NewRouter(logger, redisAddr)

	// Start server
	addr := ":" + port
	log.Printf("Voice service listening on %s", addr)
	log.Fatal(listenAndServe(addr, router))
}
