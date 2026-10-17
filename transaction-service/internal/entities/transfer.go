package entities

import "time"

type TransferStatus string

const (
	TransferStatusPending   TransferStatus = "PENDING"
	TransferStatusCompleted TransferStatus = "COMPLETED"
	TransferStatusFailed    TransferStatus = "FAILED"
)

type Transfer struct {
	ID             string         `json:"id"`
	FromAccountID  string         `json:"from_account_id"`
	ToAccountID    string         `json:"to_account_id"`
	Amount         int64          `json:"amount"`
	Currency       string         `json:"currency"`
	Status         TransferStatus `json:"status"`
	IdempotencyKey string         `json:"idempotency_key"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}
