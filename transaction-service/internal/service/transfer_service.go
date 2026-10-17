package service

import (
	"context"
	"encoding/json"
	"errors"

	"transaction-service/internal/dto"
	"transaction-service/internal/entities"
	"transaction-service/internal/event"
	"transaction-service/internal/repository"
)

var (
	ErrTransferNotFound   = repository.ErrTransferNotFound
	ErrInvalidAmount      = errors.New("amount must be greater than zero")
	ErrSameAccount        = errors.New("source and destination accounts must be different")
	ErrIdempotencyConflict = errors.New("a transfer with this idempotency key already exists")
)

type TransferService interface {
	InitiateTransfer(ctx context.Context, idempotencyKey string, req dto.CreateTransferRequest) (*entities.Transfer, error)
	GetTransfer(ctx context.Context, id string) (*entities.Transfer, error)
	UpdateStatus(ctx context.Context, id string, status entities.TransferStatus) error
}

type transferService struct {
	transferRepo repository.TransferRepository
}

func NewTransferService(transferRepo repository.TransferRepository) TransferService {
	return &transferService{
		transferRepo: transferRepo,
	}
}

func (s *transferService) InitiateTransfer(ctx context.Context, idempotencyKey string, req dto.CreateTransferRequest) (*entities.Transfer, error) {
	if req.Amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if req.FromAccountID == req.ToAccountID {
		return nil, ErrSameAccount
	}

	// 1. Проверяем идемпотентность
	existing, err := s.transferRepo.GetByIdempotencyKey(ctx, idempotencyKey)
	if err == nil && existing != nil {
		return existing, nil
	}

	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}

	transfer := &entities.Transfer{
		FromAccountID:  req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		Amount:         req.Amount,
		Currency:       currency,
		Status:         entities.TransferStatusPending,
		IdempotencyKey: idempotencyKey,
	}

	// 2. Формируем событие для Outbox
	// payload сериализуем до вставки, id проставится внутри транзакции
	reqEvent := event.TransferRequestedEvent{
		FromAccountID: req.FromAccountID,
		ToAccountID:   req.ToAccountID,
		Amount:        req.Amount,
		Currency:      currency,
	}
	payloadBytes, _ := json.Marshal(reqEvent)

	outbox := &entities.OutboxRecord{
		Topic:   "transfer.requested",
		Key:     idempotencyKey,
		Payload: payloadBytes,
	}

	// 3. Атомарно сохраняем перевод + Outbox в единой SQL-транзакции
	err = s.transferRepo.CreateWithOutbox(ctx, transfer, outbox)
	if err != nil {
		return nil, err
	}

	return transfer, nil
}

func (s *transferService) GetTransfer(ctx context.Context, id string) (*entities.Transfer, error) {
	return s.transferRepo.GetByID(ctx, id)
}

func (s *transferService) UpdateStatus(ctx context.Context, id string, status entities.TransferStatus) error {
	return s.transferRepo.UpdateStatus(ctx, id, status)
}
