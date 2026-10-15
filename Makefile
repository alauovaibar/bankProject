.PHONY: up down lint test build clean help

help:
	@echo "Available commands:"
	@echo "  make up         - Start infrastructure (Postgres, Redis, Kafka, Kafka UI)"
	@echo "  make down       - Stop infrastructure"
	@echo "  make lint       - Run golangci-lint"
	@echo "  make test       - Run unit tests across all microservices"
	@echo "  make build      - Build all microservice binaries"
	@echo "  make clean      - Clean build artifacts"

up:
	docker compose up -d

down:
	docker compose down

test:
	cd auth-service && go test -v -race ./...
	cd account-service && go test -v -race ./...
	cd transaction-service && go test -v -race ./...
	cd notification-service && go test -v -race ./...

build:
	cd auth-service && go build -o ../bin/auth-service ./cmd/api
	cd account-service && go build -o ../bin/account-service ./cmd/api
	cd transaction-service && go build -o ../bin/transaction-service ./cmd/api
	cd notification-service && go build -o ../bin/notification-service ./cmd/api

clean:
	rm -rf bin/
