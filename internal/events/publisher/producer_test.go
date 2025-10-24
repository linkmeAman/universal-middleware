package publisher_test

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/linkmeAman/universal-middleware/internal/events/publisher"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProducer(t *testing.T) {
	// Create a mock sync producer
	mockProducer := mocks.NewSyncProducer(t, nil)

	// Create a test logger
	log := logger.NewTestLogger()

	// Create producer config
	cfg := publisher.ProducerConfig{
		Brokers:           []string{"localhost:9092"},
		RequiredAcks:      sarama.WaitForAll,
		Compression:       sarama.CompressionSnappy,
		MaxRetries:        3,
		RetryBackoff:      100 * time.Millisecond,
		ConnectionTimeout: 1 * time.Second,
	}

	// Create producer with mock
	producer := &publisher.Producer{
		Producer: mockProducer,
		Log:      log,
	}

	t.Run("successful publish", func(t *testing.T) {
		// Set up expectations
		mockProducer.ExpectSendMessageWithCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
			assert.Equal(t, "test-topic", msg.Topic)
			assert.Equal(t, "test-key", string(msg.Key))
			assert.Equal(t, "test-value", string(msg.Value))
			return nil
		})

		// Publish a message
		err := producer.Publish(context.Background(), "test-topic", "test-key", []byte("test-value"))
		require.NoError(t, err)
	})

	t.Run("failed publish", func(t *testing.T) {
		// Set up expectations for failure
		mockProducer.ExpectSendMessageAndFail(sarama.ErrBrokerNotAvailable)

		// Attempt to publish
		err := producer.Publish(context.Background(), "test-topic", "test-key", []byte("test-value"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "broker not available")
	})

	t.Run("successful batch publish", func(t *testing.T) {
		messages := []publisher.Message{
			{Key: "key1", Value: []byte("value1")},
			{Key: "key2", Value: []byte("value2")},
		}

		// Set up expectations for batch
		for range messages {
			mockProducer.ExpectSendMessageWithCheckerFunctionAndSucceed(func(msg *sarama.ProducerMessage) error {
				assert.Equal(t, "test-topic", msg.Topic)
				return nil
			})
		}

		// Publish batch
		err := producer.PublishBatch(context.Background(), "test-topic", messages)
		require.NoError(t, err)
	})
}
