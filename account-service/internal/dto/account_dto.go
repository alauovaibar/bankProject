package dto

type CreateAccountRequest struct {
	Currency string `json:"currency"`
}

type AccountResponse struct {
	ID       string `json:"id"`
	UserID   string `json:"user_id"`
	Currency string `json:"currency"`
	Balance  int64  `json:"balance"`
}

type BalanceResponse struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"`
	Currency  string `json:"currency"`
}
