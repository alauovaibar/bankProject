package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"transaction-service/internal/appError"
	"transaction-service/internal/dto"
	"transaction-service/internal/service"
)

type TransferHandler struct {
	transferService service.TransferService
}

func NewTransferHandler(transferService service.TransferService) *TransferHandler {
	return &TransferHandler{
		transferService: transferService,
	}
}

func (h *TransferHandler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		appError.WriteError(w, appError.NewAppError(errors.New("bad request"), "Idempotency-Key header is required", http.StatusBadRequest))
		return
	}

	var req dto.CreateTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appError.WriteError(w, appError.NewAppError(err, "invalid json body", http.StatusBadRequest))
		return
	}

	transfer, err := h.transferService.InitiateTransfer(r.Context(), idempotencyKey, req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidAmount) || errors.Is(err, service.ErrSameAccount) {
			appError.WriteError(w, appError.NewAppError(err, err.Error(), http.StatusBadRequest))
			return
		}
		appError.WriteError(w, appError.NewAppError(err, "failed to initiate transfer: "+err.Error(), http.StatusInternalServerError))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted (Saga запущена асинхронно)
	_ = json.NewEncoder(w).Encode(transfer)
}

func (h *TransferHandler) GetTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 {
		appError.WriteError(w, appError.NewAppError(errors.New("bad request"), "transfer id is required in url path", http.StatusBadRequest))
		return
	}
	transferID := pathParts[len(pathParts)-1]

	transfer, err := h.transferService.GetTransfer(r.Context(), transferID)
	if err != nil {
		if errors.Is(err, service.ErrTransferNotFound) {
			appError.WriteError(w, appError.NewAppError(err, "transfer not found", http.StatusNotFound))
			return
		}
		appError.WriteError(w, appError.NewAppError(err, "failed to get transfer", http.StatusInternalServerError))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(transfer)
}
