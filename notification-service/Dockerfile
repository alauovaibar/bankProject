FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o app ./cmd/api

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/app .
COPY --from=builder /app/migrations ./migrations
EXPOSE 8084
CMD ["./app"]
