package consumer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ConsumerConfig holds Kafka consumer configuration
type ConsumerConfig struct {
	Brokers          []string
	GroupID          string
	Topics           []string
	InitialOffset    int64
	MinBytes         int
	MaxBytes         int
	MaxWait          time.Duration
	SessionTimeout   time.Duration
	RebalanceTimeout time.Duration
}

// Consumer handles Kafka message consumption
type Consumer struct {
	consumer sarama.ConsumerGroup
	handler  Handler
	log      *logger.Logger
	tracer   trace.Tracer
	topics   []string
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// Handler defines the interface for message handlers
type Handler interface {
	Handle(ctx context.Context, msg *sarama.ConsumerMessage) error
}

// NewConsumer creates a new Kafka consumer instance
func NewConsumer(cfg ConsumerConfig, handler Handler, log *logger.Logger) (*Consumer, error) {
	config := sarama.NewConfig()

	// Consumer group config
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = cfg.InitialOffset
	config.Consumer.MaxProcessingTime = cfg.MaxWait
	config.Consumer.Fetch.Min = int32(cfg.MinBytes)
	config.Consumer.Fetch.Max = int32(cfg.MaxBytes)

	// General config
	config.Consumer.Group.Session.Timeout = cfg.SessionTimeout
	config.Consumer.Group.Rebalance.Timeout = cfg.RebalanceTimeout

	group, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.GroupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Consumer{
		consumer: group,
		handler:  handler,
		log:      log,
		tracer:   otel.GetTracerProvider().Tracer("kafka-consumer"),
		topics:   cfg.Topics,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// Start begins consuming messages
func (c *Consumer) Start() error {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for {
			select {
			case <-c.ctx.Done():
				return
			default:
				if err := c.consumer.Consume(c.ctx, c.topics, c); err != nil {
					c.log.Error("Error from consumer", zap.Error(err))
				}
			}
		}
	}()
	return nil
}

// Stop gracefully stops the consumer
func (c *Consumer) Stop() error {
	c.cancel()
	c.wg.Wait()
	return c.consumer.Close()
}

// Setup is run at the beginning of a new session
func (c *Consumer) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is run at the end of a session
func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim handles message consumption
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		ctx := c.extractContext(msg)
		ctx, span := c.tracer.Start(ctx, "kafka.consume",
			trace.WithAttributes(
				attribute.String("messaging.system", "kafka"),
				attribute.String("messaging.destination", msg.Topic),
				attribute.String("messaging.destination_kind", "topic"),
				attribute.String("messaging.kafka.consumer_group", fmt.Sprintf("%d", session.Claims()[msg.Topic][0])),
				attribute.Int64("messaging.kafka.offset", msg.Offset),
				attribute.Int64("messaging.kafka.partition", int64(msg.Partition)),
				attribute.String("messaging.message_id", string(msg.Key)),
				attribute.Int("messaging.message_payload_size_bytes", len(msg.Value)),
			),
		)

		err := c.handler.Handle(ctx, msg)
		if err != nil {
			c.log.Error("Failed to handle message",
				zap.String("topic", msg.Topic),
				zap.Int32("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
				zap.Error(err),
			)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		} else {
			session.MarkMessage(msg, "")
		}

		span.End()
	}
	return nil
}

// extractContext extracts tracing context from message headers
func (c *Consumer) extractContext(msg *sarama.ConsumerMessage) context.Context {
	ctx := context.Background()
	propagator := otel.GetTextMapPropagator()
	// Convert headers to a map-like carrier
	carrier := propagation.HeaderCarrier{}
	for _, h := range msg.Headers {
		carrier[string(h.Key)] = []string{string(h.Value)}
	}
	return propagator.Extract(ctx, carrier)
}

// Ping checks if the consumer is running
func (c *Consumer) Ping() error {
	if c.consumer == nil {
		return fmt.Errorf("consumer not initialized")
	}

	select {
	case <-c.ctx.Done():
		return fmt.Errorf("consumer is stopped")
	default:
		return nil
	}
}
