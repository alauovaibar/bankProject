package event

type TransferCompletedNotificationEvent struct {
	TransferID string `json:"transfer_id"`
	UserID     string `json:"user_id"`
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	Status     string `json:"status"` // COMPLETED или FAILED
	Reason     string `json:"reason,omitempty"`
}
