package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"notification-service/internal/entities"
	"notification-service/internal/repository"
)

var (
	ErrDuplicateEvent = errors.New("event has already been processed")
)

type NotificationService interface {
	SendNotification(ctx context.Context, n *entities.Notification) error
	GetNotifications(ctx context.Context, userID string) ([]*entities.Notification, error)
}

type notificationService struct {
	repo     repository.NotificationRepository
	redisCli *redis.Client
	dedupTTL time.Duration
}

func NewNotificationService(repo repository.NotificationRepository, redisCli *redis.Client) NotificationService {
	return &notificationService{
		repo:     repo,
		redisCli: redisCli,
		dedupTTL: 24 * time.Hour,
	}
}

func (s *notificationService) SendNotification(ctx context.Context, n *entities.Notification) error {
	// 1. Дедупликация через Redis (SETNX по event_id)
	if s.redisCli != nil && n.EventID != "" {
		key := "dedup:event:" + n.EventID
		ok, err := s.redisCli.SetNX(ctx, key, "1", s.dedupTTL).Result()
		if err != nil {
			log.Printf("[NotificationService] redis dedup error: %v", err)
		} else if !ok {
			log.Printf("[NotificationService] duplicate event detected: %s. Skipping.", n.EventID)
			return ErrDuplicateEvent
		}
	}

	// 2. Эмуляция отправки (Email / Push / SMS)
	log.Printf("[NotificationService] SENDING %s to user %s: %s (Status: %s)",
		n.Channel, n.UserID, n.Message, n.Status)

	// 3. Сохранение записи в PostgreSQL
	err := s.repo.Save(ctx, n)
	if err != nil {
		return fmt.Errorf("failed to save notification audit: %w", err)
	}

	return nil
}

func (s *notificationService) GetNotifications(ctx context.Context, userID string) ([]*entities.Notification, error) {
	return s.repo.GetByUserID(ctx, userID)
}
