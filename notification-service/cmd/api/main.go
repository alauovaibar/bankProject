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

	"notification-service/internal/handler"
	"notification-service/internal/kafka"
	"notification-service/internal/repository"
	"notification-service/internal/service"
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func main() {
	port := getEnv("PORT", "8084")
	pgDSN := getEnv("POSTGRES_DSN", "postgres://aibar:qwerty@localhost:5435/notification-db?sslmode=disable")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	kafkaBrokers := []string{getEnv("KAFKA_BROKER", "localhost:9092")}

	log.Printf("[notification-service] Connecting to PostgreSQL...")
	db, err := sql.Open("postgres", pgDSN)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Printf("[notification-service] warning: postgres not reachable at %s: %v", pgDSN, err)
	} else {
		log.Println("[notification-service] PostgreSQL connected successfully.")
	}

	log.Printf("[notification-service] Connecting to Redis...")
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	notificationRepo := repository.NewNotificationRepository(db)
	notificationSvc := service.NewNotificationService(notificationRepo, rdb)
	notificationHandler := handler.NewNotificationHandler(notificationSvc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запуск фонового Kafka Consumer для событий переводов
	notificationConsumer := kafka.NewNotificationConsumer(
		kafkaBrokers,
		"transfer.completed",
		"notification-service-group",
		notificationSvc,
	)
	go func() {
		if err := notificationConsumer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("[notification-service] consumer stopped: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/notifications/", notificationHandler.GetUserNotifications)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"notification-service"}`))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("[notification-service] HTTP server listening on port :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[notification-service] Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	cancel() // Останавливаем Kafka Consumer
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}
	log.Println("[notification-service] Server exited cleanly.")
}
