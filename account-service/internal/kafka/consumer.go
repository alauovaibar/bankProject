package kafka

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"

	"account-service/internal/event"
	"account-service/internal/service"
)

type TransferConsumer struct {
	reader         *kafka.Reader
	writer         *kafka.Writer
	accountService service.AccountService
}

func NewTransferConsumer(
	brokers []string,
	inTopic string,
	outTopic string,
	groupID string,
	accountService service.AccountService,
) *TransferConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    inTopic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})

	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    outTopic,
		Balancer: &kafka.LeastBytes{},
	}

	return &TransferConsumer{
		reader:         reader,
		writer:         writer,
		accountService: accountService,
	}
}

func (c *TransferConsumer) Start(ctx context.Context) error {
	defer c.reader.Close()
	defer c.writer.Close()

	log.Println("[Kafka Consumer] account-service listening for transfer.requested...")

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
				log.Printf("[Kafka Consumer] error fetching message: %v", err)
				continue
			}

			var reqEvent event.TransferRequestedEvent
			if err := json.Unmarshal(msg.Value, &reqEvent); err != nil {
				log.Printf("[Kafka Consumer] unmarshal error: %v", err)
				_ = c.reader.CommitMessages(ctx, msg)
				continue
			}

			// Выполняем дебет и кредит на счетах
			transferErr := c.accountService.ProcessTransfer(
				ctx,
				reqEvent.FromAccountID,
				reqEvent.ToAccountID,
				reqEvent.Amount,
			)

			resultEvent := event.TransferResultEvent{
				TransferID: reqEvent.TransferID,
				Status:     "COMPLETED",
			}

			if transferErr != nil {
				resultEvent.Status = "FAILED"
				resultEvent.Reason = transferErr.Error()
				log.Printf("[Kafka Consumer] transfer %s failed: %v", reqEvent.TransferID, transferErr)
			} else {
				log.Printf("[Kafka Consumer] transfer %s completed successfully", reqEvent.TransferID)
			}

			// Публикуем результат в Kafka (transfer.completed или transfer.failed)
			resPayload, _ := json.Marshal(resultEvent)
			err = c.writer.WriteMessages(ctx, kafka.Message{
				Key:   []byte(reqEvent.TransferID),
				Value: resPayload,
			})
			if err != nil {
				log.Printf("[Kafka Consumer] error sending transfer result: %v", err)
			}

			// Фиксируем смещение (commit offset)
			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				log.Printf("[Kafka Consumer] commit offset error: %v", err)
			}
		}
	}
}
