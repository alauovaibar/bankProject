package repository

import (
	"context"
	"database/sql"
	"errors"

	"transaction-service/internal/entities"
)

var (
	ErrTransferNotFound = errors.New("transfer not found")
)

type TransferRepository interface {
	CreateWithOutbox(ctx context.Context, transfer *entities.Transfer, outbox *entities.OutboxRecord) error
	GetByID(ctx context.Context, id string) (*entities.Transfer, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*entities.Transfer, error)
	UpdateStatus(ctx context.Context, id string, status entities.TransferStatus) error
}

type transferRepository struct {
	db *sql.DB
}

func NewTransferRepository(db *sql.DB) TransferRepository {
	return &transferRepository{db: db}
}

func (t *transferRepository) CreateWithOutbox(ctx context.Context, transfer *entities.Transfer, outbox *entities.OutboxRecord) error {
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Вставляем перевод со статусом PENDING
	transferQuery := `
		INSERT INTO transfers (from_account_id, to_account_id, amount, currency, status, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	err = tx.QueryRowContext(ctx, transferQuery,
		transfer.FromAccountID,
		transfer.ToAccountID,
		transfer.Amount,
		transfer.Currency,
		transfer.Status,
		transfer.IdempotencyKey,
	).Scan(&transfer.ID, &transfer.CreatedAt, &transfer.UpdatedAt)
	if err != nil {
		return err
	}

	// 2. Вставляем событие в outbox в той же транзакции
	outboxQuery := `
		INSERT INTO outbox (topic, key, payload, processed)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	err = tx.QueryRowContext(ctx, outboxQuery,
		outbox.Topic,
		outbox.Key,
		outbox.Payload,
		false,
	).Scan(&outbox.ID, &outbox.CreatedAt)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (t *transferRepository) GetByID(ctx context.Context, id string) (*entities.Transfer, error) {
	query := `
		SELECT id, from_account_id, to_account_id, amount, currency, status, idempotency_key, created_at, updated_at
		FROM transfers
		WHERE id = $1
	`
	transfer := &entities.Transfer{}
	err := t.db.QueryRowContext(ctx, query, id).Scan(
		&transfer.ID,
		&transfer.FromAccountID,
		&transfer.ToAccountID,
		&transfer.Amount,
		&transfer.Currency,
		&transfer.Status,
		&transfer.IdempotencyKey,
		&transfer.CreatedAt,
		&transfer.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransferNotFound
		}
		return nil, err
	}
	return transfer, nil
}

func (t *transferRepository) GetByIdempotencyKey(ctx context.Context, key string) (*entities.Transfer, error) {
	query := `
		SELECT id, from_account_id, to_account_id, amount, currency, status, idempotency_key, created_at, updated_at
		FROM transfers
		WHERE idempotency_key = $1
	`
	transfer := &entities.Transfer{}
	err := t.db.QueryRowContext(ctx, query, key).Scan(
		&transfer.ID,
		&transfer.FromAccountID,
		&transfer.ToAccountID,
		&transfer.Amount,
		&transfer.Currency,
		&transfer.Status,
		&transfer.IdempotencyKey,
		&transfer.CreatedAt,
		&transfer.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransferNotFound
		}
		return nil, err
	}
	return transfer, nil
}

func (t *transferRepository) UpdateStatus(ctx context.Context, id string, status entities.TransferStatus) error {
	query := `
		UPDATE transfers
		SET status = $1, updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`
	res, err := t.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return ErrTransferNotFound
	}
	return nil
}
