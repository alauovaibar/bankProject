package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"account-service/internal/appError"
	"account-service/internal/dto"
	"account-service/internal/middleware"
	"account-service/internal/service"
)

type AccountHandler struct {
	accountService service.AccountService
}

func NewAccountHandler(accountService service.AccountService) *AccountHandler {
	return &AccountHandler{
		accountService: accountService,
	}
}

func (h *AccountHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := r.Context().Value(middleware.UserIDKey).(string)
	if userID == "" {
		userID = r.Header.Get("X-User-ID")
	}
	if userID == "" {
		appError.WriteError(w, appError.NewAppError(errors.New("unauthorized"), "user not authenticated", http.StatusUnauthorized))
		return
	}

	var req dto.CreateAccountRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	account, err := h.accountService.CreateAccount(r.Context(), userID, req)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(err, "failed to create account", http.StatusInternalServerError))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(account)
}

func (h *AccountHandler) GetBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 {
		appError.WriteError(w, appError.NewAppError(errors.New("bad request"), "account id is required in url path", http.StatusBadRequest))
		return
	}
	accountID := pathParts[len(pathParts)-2] // /api/v1/accounts/{id}/balance

	balance, err := h.accountService.GetBalance(r.Context(), accountID)
	if err != nil {
		if errors.Is(err, service.ErrAccountNotFound) {
			appError.WriteError(w, appError.NewAppError(err, "account not found", http.StatusNotFound))
			return
		}
		appError.WriteError(w, appError.NewAppError(err, "failed to get balance", http.StatusInternalServerError))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"account_id": accountID,
		"balance":    balance,
	})
}

func (h *AccountHandler) GetUserAccounts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, _ := r.Context().Value(middleware.UserIDKey).(string)
	if userID == "" {
		userID = r.Header.Get("X-User-ID")
	}
	if userID == "" {
		appError.WriteError(w, appError.NewAppError(errors.New("unauthorized"), "user not authenticated", http.StatusUnauthorized))
		return
	}

	accounts, err := h.accountService.GetUserAccounts(r.Context(), userID)
	if err != nil {
		appError.WriteError(w, appError.NewAppError(err, "failed to get accounts", http.StatusInternalServerError))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(accounts)
}
