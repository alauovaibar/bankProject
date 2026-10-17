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

	"account-service/internal/handler"
	"account-service/internal/kafka"
	"account-service/internal/middleware"
	"account-service/internal/repository"
	"account-service/internal/service"
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func main() {
	port := getEnv("PORT", "8082")
	pgDSN := getEnv("POSTGRES_DSN", "postgres://aibar:qwerty@localhost:5433/account-db?sslmode=disable")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	kafkaBrokers := []string{getEnv("KAFKA_BROKER", "localhost:9092")}
	jwtSecret := getEnv("JWT_SECRET", "super-secret-bank-auth-key-32-bytes!")

	log.Printf("[account-service] Connecting to PostgreSQL...")
	db, err := sql.Open("postgres", pgDSN)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Printf("[account-service] warning: postgres not reachable at %s: %v", pgDSN, err)
	} else {
		log.Println("[account-service] PostgreSQL connected successfully.")
	}

	log.Printf("[account-service] Connecting to Redis...")
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	accountRepo := repository.NewAccountRepository(db)
	cacheRepo := repository.NewCacheRepository(rdb)

	accountSvc := service.NewAccountService(accountRepo, cacheRepo)
	accountHandler := handler.NewAccountHandler(accountSvc)

	// Запуск фонового Kafka Consumer для саги переводов
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	transferConsumer := kafka.NewTransferConsumer(
		kafkaBrokers,
		"transfer.requested",
		"transfer.completed",
		"account-service-group",
		accountSvc,
	)

	go func() {
		if err := transferConsumer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("[account-service] kafka consumer stopped: %v", err)
		}
	}()

	authMW := middleware.AuthMiddleware(jwtSecret)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/accounts", authMW(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			accountHandler.CreateAccount(w, r)
		} else if r.Method == http.MethodGet {
			accountHandler.GetUserAccounts(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	mux.Handle("/api/v1/accounts/", authMW(http.HandlerFunc(accountHandler.GetBalance)))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"account-service"}`))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("[account-service] HTTP server listening on port :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[account-service] Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	cancel() // Останавливаем Kafka Consumer
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}
	log.Println("[account-service] Server exited cleanly.")
}
