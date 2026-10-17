package entities

import "time"

type Account struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Currency  string    `json:"currency"`
	Balance   int64     `json:"balance"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TransactionHistory struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	Amount    int64     `json:"amount"`
	Type      string    `json:"type"` // "DEBIT" или "CREDIT"
	CreatedAt time.Time `json:"created_at"`
}
