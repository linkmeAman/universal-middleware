package consumer

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/linkmeAman/universal-middleware/internal/events/schemas"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// DeadLetterConfig holds configuration for dead letter queue
type DeadLetterConfig struct {
	Topic          string
	MaxRetries     int
	RetryBackoff   time.Duration
	ErrorThreshold int
}

// DeadLetterHandler handles messages that failed processing
type DeadLetterHandler struct {
	config   DeadLetterConfig
	producer sarama.SyncProducer
	log      *logger.Logger
	tracer   trace.Tracer
}

// NewDeadLetterHandler creates a new dead letter handler
func NewDeadLetterHandler(cfg DeadLetterConfig, producer sarama.SyncProducer, log *logger.Logger) *DeadLetterHandler {
	return &DeadLetterHandler{
		config:   cfg,
		producer: producer,
		log:      log,
		tracer:   otel.GetTracerProvider().Tracer("kafka-dlq"),
	}
}

// HandleFailedMessage processes a failed message
func (h *DeadLetterHandler) HandleFailedMessage(ctx context.Context, msg *sarama.ConsumerMessage, err error) error {
	ctx, span := h.tracer.Start(ctx, "dlq.handle_failed_message",
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", msg.Topic),
			attribute.String("messaging.message_id", string(msg.Key)),
			attribute.String("error", err.Error()),
		),
	)
	defer span.End()

	// Extract retry count from headers
	retryCount := 0
	for _, header := range msg.Headers {
		if string(header.Key) == "retry-count" {
			retryCount = int(header.Value[0])
			break
		}
	}

	// Check if we should retry or move to DLQ
	if retryCount < h.config.MaxRetries {
		return h.retryMessage(ctx, msg, retryCount+1)
	}

	return h.moveToDeadLetter(ctx, msg, err)
}

func (h *DeadLetterHandler) retryMessage(ctx context.Context, msg *sarama.ConsumerMessage, retryCount int) error {
	// Create retry message with incremented count
	retryMsg := &sarama.ProducerMessage{
		Topic: msg.Topic,
		Key:   sarama.StringEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Value),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("retry-count"),
				Value: []byte{byte(retryCount)},
			},
		},
	}

	// Copy original headers
	for _, header := range msg.Headers {
		if string(header.Key) != "retry-count" {
			retryMsg.Headers = append(retryMsg.Headers, sarama.RecordHeader{
				Key:   header.Key,
				Value: header.Value,
			})
		}
	}

	// Add retry delay using message timestamp
	retryMsg.Timestamp = time.Now().Add(time.Duration(retryCount) * h.config.RetryBackoff)

	partition, offset, err := h.producer.SendMessage(retryMsg)
	if err != nil {
		h.log.Error("Failed to send retry message",
			zap.String("topic", msg.Topic),
			zap.String("key", string(msg.Key)),
			zap.Int("retry_count", retryCount),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send retry message: %w", err)
	}

	h.log.Debug("Message scheduled for retry",
		zap.String("topic", msg.Topic),
		zap.String("key", string(msg.Key)),
		zap.Int("retry_count", retryCount),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
	)

	return nil
}

// convertHeaders converts Kafka record headers to the correct type
func convertHeaders(headers []*sarama.RecordHeader) []sarama.RecordHeader {
	converted := make([]sarama.RecordHeader, len(headers))
	for i, header := range headers {
		converted[i] = sarama.RecordHeader{
			Key:   header.Key,
			Value: header.Value,
		}
	}
	return converted
}

func (h *DeadLetterHandler) moveToDeadLetter(ctx context.Context, msg *sarama.ConsumerMessage, originalErr error) error {
	// Create dead letter event
	deadLetterEvent := schemas.Event{
		Type:   schemas.EventTypeMessageDeadLettered,
		Source: "kafka-consumer",
		Time:   time.Now(),
		Data: map[string]interface{}{
			"original_topic":     msg.Topic,
			"original_partition": msg.Partition,
			"original_offset":    msg.Offset,
			"error":              originalErr.Error(),
			"headers":            msg.Headers,
		},
	}

	// Serialize event
	eventBytes, err := deadLetterEvent.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal dead letter event: %w", err)
	}

	// Create dead letter message
	dlqMsg := &sarama.ProducerMessage{
		Topic: h.config.Topic,
		Key:   sarama.StringEncoder(msg.Key),
		Value: sarama.ByteEncoder(eventBytes),
		Headers: append(convertHeaders(msg.Headers), sarama.RecordHeader{
			Key:   []byte("original_topic"),
			Value: []byte(msg.Topic),
		}),
	}

	// Send to dead letter queue
	partition, offset, err := h.producer.SendMessage(dlqMsg)
	if err != nil {
		h.log.Error("Failed to send message to dead letter queue",
			zap.String("topic", msg.Topic),
			zap.String("key", string(msg.Key)),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send to dead letter queue: %w", err)
	}

	h.log.Info("Message moved to dead letter queue",
		zap.String("original_topic", msg.Topic),
		zap.String("key", string(msg.Key)),
		zap.String("dlq_topic", h.config.Topic),
		zap.Int32("dlq_partition", partition),
		zap.Int64("dlq_offset", offset),
	)

	return nil
}
