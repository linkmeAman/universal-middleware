package processor

import (
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/linkmeAman/universal-middleware/internal/events/consumer"
	"github.com/linkmeAman/universal-middleware/internal/events/publisher"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
)

// Service handles event processing
type Service struct {
	consumer  *consumer.Consumer
	publisher *publisher.Producer
	log       *logger.Logger
}

// NewService creates a new processor service
func NewService(brokers []string, topic string, groupID string, minBytes int, maxBytes int, log *logger.Logger) (*Service, error) {
	// Create event consumer with min/max bytes and initial offset
	consumer, err := consumer.NewConsumer(consumer.ConsumerConfig{
		Brokers:          brokers,
		Topics:           []string{topic},
		GroupID:          groupID,
		MinBytes:         minBytes,
		MaxBytes:         maxBytes,
		MaxWait:          5 * time.Second,
		InitialOffset:    sarama.OffsetNewest, // Start from newest messages
		SessionTimeout:   10 * time.Second,
		RebalanceTimeout: 15 * time.Second,
	}, nil, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	// Create event publisher for downstream events
	pub, err := publisher.NewProducer(publisher.ProducerConfig{
		Brokers:           brokers,
		RequiredAcks:      sarama.WaitForAll, // Required for idempotent producer
		MaxRetries:        3,
		RetryBackoff:      time.Second,
		ConnectionTimeout: 5 * time.Second,
	}, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher: %w", err)
	}

	return &Service{
		consumer:  consumer,
		publisher: pub,
		log:       log,
	}, nil
}

// Start begins processing events
func (s *Service) Start() error {
	return s.consumer.Start()
}

// Stop gracefully shuts down the service
func (s *Service) Stop() error {
	// Stop consumer first
	if err := s.consumer.Stop(); err != nil {
		return err
	}

	// Stop publisher
	return s.publisher.Close()
}
