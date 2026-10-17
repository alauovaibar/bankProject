package event

type TransferRequestedEvent struct {
	TransferID    string `json:"transfer_id"`
	FromAccountID string `json:"from_account_id"`
	ToAccountID   string `json:"to_account_id"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
}

type TransferResultEvent struct {
	TransferID string `json:"transfer_id"`
	Status     string `json:"status"` // COMPLETED или FAILED
	Reason     string `json:"reason,omitempty"`
}
