package repository

import (
	"context"
	"database/sql"
	"errors"

	"account-service/internal/entities"
)

var (
	ErrAccountNotFound   = errors.New("account not found")
	ErrInsufficientFunds = errors.New("insufficient funds")
)

type AccountRepository interface {
	Create(ctx context.Context, account *entities.Account) error
	GetByID(ctx context.Context, id string) (*entities.Account, error)
	GetByUserID(ctx context.Context, userID string) ([]*entities.Account, error)
	ExecuteTransfer(ctx context.Context, fromID, toID string, amount int64) error
	AddHistory(ctx context.Context, history *entities.TransactionHistory) error
}

type accountRepository struct {
	db *sql.DB
}

func NewAccountRepository(db *sql.DB) AccountRepository {
	return &accountRepository{db: db}
}

func (r *accountRepository) Create(ctx context.Context, account *entities.Account) error {
	query := `
		INSERT INTO accounts (user_id, currency, balance, version)
		VALUES ($1, $2, $3, $4)
		RETURNING id, balance, version, created_at, updated_at
	`
	return r.db.QueryRowContext(ctx, query,
		account.UserID,
		account.Currency,
		account.Balance,
		1,
	).Scan(
		&account.ID,
		&account.Balance,
		&account.Version,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
}

func (r *accountRepository) GetByID(ctx context.Context, id string) (*entities.Account, error) {
	query := `
		SELECT id, user_id, currency, balance, version, created_at, updated_at
		FROM accounts
		WHERE id = $1
	`
	account := &entities.Account{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&account.ID,
		&account.UserID,
		&account.Currency,
		&account.Balance,
		&account.Version,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}
	return account, nil
}

func (r *accountRepository) GetByUserID(ctx context.Context, userID string) ([]*entities.Account, error) {
	query := `
		SELECT id, user_id, currency, balance, version, created_at, updated_at
		FROM accounts
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*entities.Account
	for rows.Next() {
		var acc entities.Account
		err = rows.Scan(
			&acc.ID,
			&acc.UserID,
			&acc.Currency,
			&acc.Balance,
			&acc.Version,
			&acc.CreatedAt,
			&acc.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, &acc)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return accounts, nil
}

func (r *accountRepository) ExecuteTransfer(ctx context.Context, fromID, toID string, amount int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Блокируем счет списания с проверкой баланса
	var currentBalance int64
	lockQuery := `SELECT balance FROM accounts WHERE id = $1 FOR UPDATE`
	err = tx.QueryRowContext(ctx, lockQuery, fromID).Scan(&currentBalance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAccountNotFound
		}
		return err
	}

	if currentBalance < amount {
		return ErrInsufficientFunds
	}

	// Списание
	debitQuery := `
		UPDATE accounts
		SET balance = balance - $1, version = version + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	res, err := tx.ExecContext(ctx, debitQuery, amount, fromID)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrAccountNotFound
	}

	// Зачисление
	creditQuery := `
		UPDATE accounts
		SET balance = balance + $1, version = version + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	res, err = tx.ExecContext(ctx, creditQuery, amount, toID)
	if err != nil {
		return err
	}
	rowsAffected, _ = res.RowsAffected()
	if rowsAffected == 0 {
		return ErrAccountNotFound
	}

	return tx.Commit()
}

func (r *accountRepository) AddHistory(ctx context.Context, history *entities.TransactionHistory) error {
	query := `
		INSERT INTO transaction_history (account_id, amount, type)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	return r.db.QueryRowContext(ctx, query,
		history.AccountID,
		history.Amount,
		history.Type,
	).Scan(&history.ID, &history.CreatedAt)
}
