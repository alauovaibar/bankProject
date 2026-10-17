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

	"transaction-service/internal/handler"
	"transaction-service/internal/kafka"
	"transaction-service/internal/middleware"
	"transaction-service/internal/repository"
	"transaction-service/internal/service"
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func main() {
	port := getEnv("PORT", "8083")
	pgDSN := getEnv("POSTGRES_DSN", "postgres://aibar:qwerty@localhost:5434/transaction-db?sslmode=disable")
	kafkaBrokers := []string{getEnv("KAFKA_BROKER", "localhost:9092")}
	jwtSecret := getEnv("JWT_SECRET", "super-secret-bank-auth-key-32-bytes!")

	log.Printf("[transaction-service] Connecting to PostgreSQL...")
	db, err := sql.Open("postgres", pgDSN)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Printf("[transaction-service] warning: postgres not reachable at %s: %v", pgDSN, err)
	} else {
		log.Println("[transaction-service] PostgreSQL connected successfully.")
	}

	transferRepo := repository.NewTransferRepository(db)
	outboxRepo := repository.NewOutboxRepository(db)

	transferSvc := service.NewTransferService(transferRepo)
	transferHandler := handler.NewTransferHandler(transferSvc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Запуск фонового Outbox Publisher (опрос таблицы outbox -> Kafka)
	outboxPublisher := kafka.NewOutboxPublisher(
		kafkaBrokers,
		outboxRepo,
		50,             // batch size
		1*time.Second,  // poll period
	)
	go func() {
		if err := outboxPublisher.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("[transaction-service] outbox publisher stopped: %v", err)
		}
	}()

	// 2. Запуск фонового Result Consumer (ответ от account-service -> обновление статуса)
	resultConsumer := kafka.NewResultConsumer(
		kafkaBrokers,
		"transfer.completed",
		"transaction-service-group",
		transferSvc,
	)
	go func() {
		if err := resultConsumer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("[transaction-service] result consumer stopped: %v", err)
		}
	}()

	authMW := middleware.AuthMiddleware(jwtSecret)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/transfers", authMW(http.HandlerFunc(transferHandler.CreateTransfer)))
	mux.Handle("/api/v1/transfers/", authMW(http.HandlerFunc(transferHandler.GetTransfer)))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"transaction-service"}`))
	})

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		log.Printf("[transaction-service] HTTP server listening on port :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[transaction-service] Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	cancel() // Останавливаем Kafka фоновые процессы
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}
	log.Println("[transaction-service] Server exited cleanly.")
}
