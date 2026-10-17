package service

import (
	"context"
	"errors"
	"time"

	"account-service/internal/dto"
	"account-service/internal/entities"
	"account-service/internal/repository"
)

var (
	ErrAccountNotFound   = repository.ErrAccountNotFound
	ErrInsufficientFunds = repository.ErrInsufficientFunds
	ErrInvalidAmount     = errors.New("amount must be greater than zero")
)

type AccountService interface {
	CreateAccount(ctx context.Context, userID string, req dto.CreateAccountRequest) (*entities.Account, error)
	GetBalance(ctx context.Context, accountID string) (int64, error)
	GetUserAccounts(ctx context.Context, userID string) ([]*entities.Account, error)
	ProcessTransfer(ctx context.Context, fromID, toID string, amount int64) error
}

type accountService struct {
	repo     repository.AccountRepository
	cache    repository.CacheRepository
	cacheTTL time.Duration
}

func NewAccountService(repo repository.AccountRepository, cache repository.CacheRepository) AccountService {
	return &accountService{
		repo:     repo,
		cache:    cache,
		cacheTTL: 5 * time.Minute,
	}
}

func (s *accountService) CreateAccount(ctx context.Context, userID string, req dto.CreateAccountRequest) (*entities.Account, error) {
	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}

	account := &entities.Account{
		UserID:   userID,
		Currency: currency,
		Balance:  0,
	}

	err := s.repo.Create(ctx, account)
	if err != nil {
		return nil, err
	}

	_ = s.cache.SetBalance(ctx, account.ID, 0, s.cacheTTL)
	return account, nil
}

func (s *accountService) GetBalance(ctx context.Context, accountID string) (int64, error) {
	// 1. Пытаемся взять баланс из кэша Redis
	balance, found, err := s.cache.GetBalance(ctx, accountID)
	if err == nil && found {
		return balance, nil
	}

	// 2. Если в кэше нет — идем в БД
	account, err := s.repo.GetByID(ctx, accountID)
	if err != nil {
		return 0, err
	}

	// 3. Сохраняем в кэш
	_ = s.cache.SetBalance(ctx, accountID, account.Balance, s.cacheTTL)

	return account.Balance, nil
}

func (s *accountService) GetUserAccounts(ctx context.Context, userID string) ([]*entities.Account, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *accountService) ProcessTransfer(ctx context.Context, fromID, toID string, amount int64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	// Выполняем атомарный перевод в БД
	err := s.repo.ExecuteTransfer(ctx, fromID, toID, amount)
	if err != nil {
		return err
	}

	// Записываем операции в историю транзакций
	_ = s.repo.AddHistory(ctx, &entities.TransactionHistory{
		AccountID: fromID,
		Amount:    amount,
		Type:      "DEBIT",
	})
	_ = s.repo.AddHistory(ctx, &entities.TransactionHistory{
		AccountID: toID,
		Amount:    amount,
		Type:      "CREDIT",
	})

	// Инвалидируем кэш балансов обоих счетов
	_ = s.cache.InvalidateBalance(ctx, fromID)
	_ = s.cache.InvalidateBalance(ctx, toID)

	return nil
}
