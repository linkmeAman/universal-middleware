package consumer_test

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/linkmeAman/universal-middleware/internal/events/consumer"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHandler implements the Handler interface for testing
type MockHandler struct {
	messages []*sarama.ConsumerMessage
}

func (h *MockHandler) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	h.messages = append(h.messages, msg)
	return nil
}

func TestConsumer(t *testing.T) {
	// Create mock consumer group
	config := sarama.NewConfig()
	mockConsumer := mocks.NewConsumerGroup()

	// Create test logger
	log := logger.NewTestLogger()

	// Create consumer config
	cfg := consumer.ConsumerConfig{
		Brokers:          []string{"localhost:9092"},
		GroupID:          "test-group",
		Topics:           []string{"test-topic"},
		InitialOffset:    sarama.OffsetOldest,
		MinBytes:         10e3,
		MaxBytes:         10e6,
		MaxWait:          500 * time.Millisecond,
		SessionTimeout:   10 * time.Second,
		RebalanceTimeout: 60 * time.Second,
	}

	// Create mock handler
	handler := &MockHandler{}

	t.Run("successful consumption", func(t *testing.T) {
		// Create test messages
		testMessages := []*sarama.ConsumerMessage{
			{
				Topic: "test-topic",
				Key:   []byte("key1"),
				Value: []byte("value1"),
			},
			{
				Topic: "test-topic",
				Key:   []byte("key2"),
				Value: []byte("value2"),
			},
		}

		// Set up mock expectations
		mockConsumer.ExpectConsumePartition("test-topic", 0, sarama.OffsetOldest).YieldMessage(testMessages...)

		// Create consumer
		consumer, err := consumer.NewConsumer(cfg, handler, log)
		require.NoError(t, err)

		// Start consuming
		err = consumer.Start()
		require.NoError(t, err)

		// Wait for messages to be processed
		time.Sleep(100 * time.Millisecond)

		// Stop consumer
		err = consumer.Stop()
		require.NoError(t, err)

		// Verify messages were handled
		assert.Len(t, handler.messages, len(testMessages))
		for i, msg := range handler.messages {
			assert.Equal(t, testMessages[i].Key, msg.Key)
			assert.Equal(t, testMessages[i].Value, msg.Value)
		}
	})

	t.Run("consumer error handling", func(t *testing.T) {
		// Set up mock to return error
		mockConsumer.ExpectError(sarama.ErrOutOfBrokers)

		// Create consumer
		consumer, err := consumer.NewConsumer(cfg, handler, log)
		require.NoError(t, err)

		// Start consuming
		err = consumer.Start()
		require.NoError(t, err)

		// Wait for error to be processed
		time.Sleep(100 * time.Millisecond)

		// Stop consumer
		err = consumer.Stop()
		require.NoError(t, err)
	})
}
