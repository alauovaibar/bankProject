package repository

import (
	"context"
	"database/sql"

	"github.com/lib/pq"
	"transaction-service/internal/entities"
)

type OutboxRepository interface {
	FetchUnprocessed(ctx context.Context, batchSize int) ([]*entities.OutboxRecord, error)
	MarkAsProcessed(ctx context.Context, ids []string) error
}

type outboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) OutboxRepository {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) FetchUnprocessed(ctx context.Context, batchSize int) ([]*entities.OutboxRecord, error) {
	query := `
		SELECT id, topic, key, payload, processed, created_at
		FROM outbox
		WHERE processed = false
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*entities.OutboxRecord
	for rows.Next() {
		var rec entities.OutboxRecord
		err = rows.Scan(
			&rec.ID,
			&rec.Topic,
			&rec.Key,
			&rec.Payload,
			&rec.Processed,
			&rec.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		records = append(records, &rec)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

func (r *outboxRepository) MarkAsProcessed(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	query := `
		UPDATE outbox
		SET processed = true
		WHERE id = ANY($1)
	`
	_, err := r.db.ExecContext(ctx, query, pq.Array(ids))
	return err
}
