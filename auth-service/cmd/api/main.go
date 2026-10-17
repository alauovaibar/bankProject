package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"

	"auth-service/internal/handler"
	"auth-service/internal/repository"
	"auth-service/internal/service"
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func main() {
	port := getEnv("PORT", "8081")
	pgDSN := getEnv("POSTGRES_DSN", "postgres://aibar:qwerty@localhost:5432/auth-db?sslmode=disable")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	jwtSecret := getEnv("JWT_SECRET", "super-secret-bank-auth-key-32-bytes!")

	log.Printf("[auth-service] Connecting to PostgreSQL...")
	db, err := sql.Open("postgres", pgDSN)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Printf("[auth-service] warning: postgres not reachable at %s: %v", pgDSN, err)
	} else {
		log.Println("[auth-service] PostgreSQL connected successfully.")
	}

	log.Printf("[auth-service] Connecting to Redis...")
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(rdb)

	authSvc := service.NewAuthService(
		userRepo,
		sessionRepo,
		jwtSecret,
		15*time.Minute,
		7*24*time.Hour,
	)

	authHandler := handler.NewAuthHandler(authSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/register", authHandler.Register)
	mux.HandleFunc("/api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("/api/v1/auth/refresh", authHandler.Refresh)
	mux.HandleFunc("/api/v1/auth/logout", authHandler.Logout)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"auth-service"}`))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("[auth-service] HTTP server listening on port :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[auth-service] Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}
	log.Println("[auth-service] Server exited cleanly.")
}
