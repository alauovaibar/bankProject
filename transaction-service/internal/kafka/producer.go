package kafka

import (
	"context"
	"log"
	"time"

	"github.com/segmentio/kafka-go"

	"transaction-service/internal/repository"
)

type OutboxPublisher struct {
	writer     *kafka.Writer
	outboxRepo repository.OutboxRepository
	batchSize  int
	pollPeriod time.Duration
}

func NewOutboxPublisher(
	brokers []string,
	outboxRepo repository.OutboxRepository,
	batchSize int,
	pollPeriod time.Duration,
) *OutboxPublisher {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Balancer: &kafka.LeastBytes{},
	}

	return &OutboxPublisher{
		writer:     writer,
		outboxRepo: outboxRepo,
		batchSize:  batchSize,
		pollPeriod: pollPeriod,
	}
}

func (p *OutboxPublisher) Start(ctx context.Context) error {
	defer p.writer.Close()

	ticker := time.NewTicker(p.pollPeriod)
	defer ticker.Stop()

	log.Println("[Outbox Publisher] started polling outbox table...")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			records, err := p.outboxRepo.FetchUnprocessed(ctx, p.batchSize)
			if err != nil {
				log.Printf("[Outbox Publisher] error fetching unprocessed: %v", err)
				continue
			}

			if len(records) == 0 {
				continue
			}

			var messages []kafka.Message
			var processedIDs []string

			for _, rec := range records {
				messages = append(messages, kafka.Message{
					Topic: rec.Topic,
					Key:   []byte(rec.Key),
					Value: rec.Payload,
				})
				processedIDs = append(processedIDs, rec.ID)
			}

			// Отправляем пачку в Kafka
			err = p.writer.WriteMessages(ctx, messages...)
			if err != nil {
				log.Printf("[Outbox Publisher] error publishing to kafka: %v", err)
				continue
			}

			// Помечаем как обработанные
			err = p.outboxRepo.MarkAsProcessed(ctx, processedIDs)
			if err != nil {
				log.Printf("[Outbox Publisher] error marking processed: %v", err)
				continue
			}

			log.Printf("[Outbox Publisher] published %d messages to Kafka", len(messages))
		}
	}
}
