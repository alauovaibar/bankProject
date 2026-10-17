package dto

type NotificationResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Message   string `json:"message"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}
