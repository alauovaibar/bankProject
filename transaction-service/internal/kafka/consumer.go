package kafka

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"

	"transaction-service/internal/entities"
	"transaction-service/internal/event"
	"transaction-service/internal/service"
)

type ResultConsumer struct {
	reader          *kafka.Reader
	transferService service.TransferService
}

func NewResultConsumer(
	brokers []string,
	topic string,
	groupID string,
	transferService service.TransferService,
) *ResultConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    topic,
		MinBytes: 10e3,
		MaxBytes: 10e6,
	})

	return &ResultConsumer{
		reader:          reader,
		transferService: transferService,
	}
}

func (c *ResultConsumer) Start(ctx context.Context) error {
	defer c.reader.Close()

	log.Println("[Kafka Consumer] transaction-service listening for transfer.results...")

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
				log.Printf("[Kafka ResultConsumer] error fetching message: %v", err)
				continue
			}

			var resEvent event.TransferResultEvent
			if err := json.Unmarshal(msg.Value, &resEvent); err != nil {
				log.Printf("[Kafka ResultConsumer] unmarshal error: %v", err)
				_ = c.reader.CommitMessages(ctx, msg)
				continue
			}

			status := entities.TransferStatusCompleted
			if resEvent.Status == "FAILED" {
				status = entities.TransferStatusFailed
			}

			// Обновляем статус перевода в базе данных
			err = c.transferService.UpdateStatus(ctx, resEvent.TransferID, status)
			if err != nil {
				log.Printf("[Kafka ResultConsumer] error updating transfer %s status: %v", resEvent.TransferID, err)
			} else {
				log.Printf("[Kafka ResultConsumer] transfer %s status updated to %s", resEvent.TransferID, status)
			}

			// Фиксируем оффсет
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[Kafka ResultConsumer] commit error: %v", err)
			}
		}
	}
}
