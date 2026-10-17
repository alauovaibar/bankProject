package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"

	"notification-service/internal/entities"
	"notification-service/internal/event"
	"notification-service/internal/service"
)

type NotificationConsumer struct {
	reader              *kafka.Reader
	notificationService service.NotificationService
}

func NewNotificationConsumer(
	brokers []string,
	topic string,
	groupID string,
	notificationService service.NotificationService,
) *NotificationConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    topic,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	return &NotificationConsumer{
		reader:              reader,
		notificationService: notificationService,
	}
}

func (c *NotificationConsumer) Start(ctx context.Context) error {
	defer c.reader.Close()

	log.Println("[Kafka Consumer] notification-service listening for transfer notifications...")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				log.Printf("[Kafka NotificationConsumer] error fetching message: %v", err)
				continue
			}

			var evt event.TransferCompletedNotificationEvent
			if err := json.Unmarshal(msg.Value, &evt); err != nil {
				log.Printf("[Kafka NotificationConsumer] unmarshal error: %v", err)
				_ = c.reader.CommitMessages(ctx, msg)
				continue
			}

			messageText := fmt.Sprintf("Your transfer of %d %s was completed successfully.", evt.Amount, evt.Currency)
			if evt.Status == "FAILED" {
				messageText = fmt.Sprintf("Your transfer of %d %s failed. Reason: %s", evt.Amount, evt.Currency, evt.Reason)
			}

			notification := &entities.Notification{
				UserID:  evt.UserID,
				EventID: evt.TransferID,
				Channel: "PUSH",
				Message: messageText,
				Status:  evt.Status,
			}

			err = c.notificationService.SendNotification(ctx, notification)
			if err != nil {
				log.Printf("[Kafka NotificationConsumer] error processing notification: %v", err)
			}

			// Фиксируем смещение
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[Kafka NotificationConsumer] commit error: %v", err)
			}
		}
	}
}
