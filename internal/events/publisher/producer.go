package publisher

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ProducerConfig holds Kafka producer configuration
type ProducerConfig struct {
	Brokers           []string
	RequiredAcks      sarama.RequiredAcks
	Compression       sarama.CompressionCodec
	MaxRetries        int
	RetryBackoff      time.Duration
	ConnectionTimeout time.Duration
}

// Producer handles Kafka message production
type Producer struct {
	producer sarama.SyncProducer
	log      *logger.Logger
	tracer   trace.Tracer
}

// NewProducer creates a new Kafka producer instance
func NewProducer(cfg ProducerConfig, log *logger.Logger) (*Producer, error) {
	config := sarama.NewConfig()

	// Producer config
	config.Producer.RequiredAcks = cfg.RequiredAcks
	config.Producer.Compression = cfg.Compression
	config.Producer.Retry.Max = cfg.MaxRetries
	config.Producer.Retry.Backoff = cfg.RetryBackoff

	// General config
	config.Net.DialTimeout = cfg.ConnectionTimeout
	config.Net.ReadTimeout = cfg.ConnectionTimeout
	config.Net.WriteTimeout = cfg.ConnectionTimeout

	// Enable idempotent delivery
	config.Producer.Idempotent = true
	config.Net.MaxOpenRequests = 1
	config.Producer.Return.Successes = true

	producer, err := sarama.NewSyncProducer(cfg.Brokers, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	return &Producer{
		producer: producer,
		log:      log,
		tracer:   trace.NewNoopTracerProvider().Tracer("kafka-producer"),
	}, nil
}

// Publish sends a message to a Kafka topic
func (p *Producer) Publish(ctx context.Context, topic string, key string, value []byte) error {
	ctx, span := p.tracer.Start(ctx, "kafka.publish",
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", topic),
			attribute.String("messaging.destination_kind", "topic"),
			attribute.String("messaging.message_id", key),
			attribute.Int("messaging.message_payload_size_bytes", len(value)),
		),
	)
	defer span.End()

	// Inject tracing context into message headers
	headers := make([]sarama.RecordHeader, 0)
	// Add any custom headers if needed
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		headers = append(headers, sarama.RecordHeader{
			Key:   []byte("trace_id"),
			Value: []byte(span.SpanContext().TraceID().String()),
		})
	}

	msg := &sarama.ProducerMessage{
		Topic:   topic,
		Key:     sarama.StringEncoder(key),
		Value:   sarama.ByteEncoder(value),
		Headers: headers,
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.log.Error("Failed to publish message",
			zap.String("topic", topic),
			zap.String("key", key),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to publish message: %w", err)
	}

	span.SetAttributes(
		attribute.Int64("messaging.kafka.partition", int64(partition)),
		attribute.Int64("messaging.kafka.offset", offset),
	)

	p.log.Debug("Message published successfully",
		zap.String("topic", topic),
		zap.String("key", key),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
	)

	return nil
}

// PublishBatch sends multiple messages to Kafka in a batch
func (p *Producer) PublishBatch(ctx context.Context, topic string, messages []Message) error {
	ctx, span := p.tracer.Start(ctx, "kafka.publishBatch",
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", topic),
			attribute.String("messaging.destination_kind", "topic"),
			attribute.Int("messaging.batch_size", len(messages)),
		),
	)
	defer span.End()

	// Prepare batch
	batch := make([]*sarama.ProducerMessage, len(messages))
	// No propagator needed, using simple trace ID header

	for i, msg := range messages {
		headers := make([]sarama.RecordHeader, 0)
		// Add trace headers if tracing is enabled
		if span := trace.SpanFromContext(ctx); span.IsRecording() {
			headers = append(headers, sarama.RecordHeader{
				Key:   []byte("trace_id"),
				Value: []byte(span.SpanContext().TraceID().String()),
			})
		}

		batch[i] = &sarama.ProducerMessage{
			Topic:   topic,
			Key:     sarama.StringEncoder(msg.Key),
			Value:   sarama.ByteEncoder(msg.Value),
			Headers: headers,
		}
	}

	// Send batch
	err := p.producer.SendMessages(batch)
	if err != nil {
		p.log.Error("Failed to publish message batch",
			zap.String("topic", topic),
			zap.Int("batch_size", len(messages)),
			zap.Error(err),
		)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("failed to publish message batch: %w", err)
	}

	p.log.Debug("Message batch published successfully",
		zap.String("topic", topic),
		zap.Int("batch_size", len(messages)),
	)

	return nil
}

// Close closes the Kafka producer
func (p *Producer) Close() error {
	if err := p.producer.Close(); err != nil {
		p.log.Error("Failed to close Kafka producer", zap.Error(err))
		return fmt.Errorf("failed to close Kafka producer: %w", err)
	}
	return nil
}

// Message represents a Kafka message
type Message struct {
	Key   string
	Value []byte
}

// Ping checks if the producer can connect to Kafka brokers
func (p *Producer) Ping() error {
	// Try to send a test message to check connectivity
	msg := &sarama.ProducerMessage{
		Topic: "__health_check",
		Value: sarama.StringEncoder("ping"),
	}

	// Create test topic if it doesn't exist
	if _, _, err := p.producer.SendMessage(msg); err != nil {
		// If the error is TopicNotFound, it means we can connect to Kafka
		if err.Error() == "Topic not found" {
			return nil
		}
		return fmt.Errorf("failed to ping Kafka: %w", err)
	}
	return nil
}
