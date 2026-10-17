package repository

import (
	"context"
	"database/sql"

	"notification-service/internal/entities"
)

type NotificationRepository interface {
	Save(ctx context.Context, n *entities.Notification) error
	GetByUserID(ctx context.Context, userID string) ([]*entities.Notification, error)
}

type notificationRepository struct {
	db *sql.DB
}

func NewNotificationRepository(db *sql.DB) *notificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) Save(ctx context.Context, n *entities.Notification) error {
	query := `INSERT INTO notifications(user_id,event_id,channel,message,status,created_at) VALUES($1,$2,$3,$4,$5,$6)RETURNING id,created_at`
	err := r.db.QueryRowContext(ctx, query, n.UserID, n.EventID, n.Channel, n.Message, n.Status).Scan(&n.ID, &n.CreatedAt)
	if err != nil {
		return err
	}
	return nil
}

func (r *notificationRepository) GetByUserID(ctx context.Context, userID string) ([]*entities.Notification, error) {
	query := `SELECT id,user_id,event_id,channel,message,status,created_at FROM notifications WHERE user_id =$1 ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []*entities.Notification

	for rows.Next() {
		var n entities.Notification
		err = rows.Scan(&n.ID, &n.UserID, &n.EventID, &n.Channel, &n.Message, &n.Status, &n.CreatedAt)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, &n)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return notifications, nil
}
