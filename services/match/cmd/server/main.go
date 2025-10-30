package main

import (
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"match/internal/match_management"
	"match/internal/metrics"
	"match/internal/routers"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

const defaultRedisAddr = "redis:6379"

func main() {
	rand.Seed(time.Now().UnixNano())

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "your-secret-key"
	}
	jwtSecret := []byte(secret)

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = defaultRedisAddr
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	mm := match_management.NewMatchManager(jwtSecret, rdb)

	// Start background processes
	go mm.SubscribeToRedis()
	go mm.StartMatchmakingLoop()
	go mm.StartPendingMatchExpirationLoop()

	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization", "Upgrade", "Connection"},
		ExposedHeaders:   []string{"Upgrade", "Connection"},
		AllowCredentials: true,
	}))

	r.Use(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Logger,
		middleware.Recoverer,
		middleware.Timeout(60*time.Second),
	)

	routers.MatchRoutes(r, mm)
	r.Handle("/api/v1/match/metrics", metrics.Handler())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("Listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
