package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/linkmeAman/universal-middleware/internal/events/publisher"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ProcessorConfig holds configuration for the outbox processor
type ProcessorConfig struct {
	BatchSize       int
	PollingInterval time.Duration
	RetryDelay      time.Duration
	MaxRetries      int
	CleanupInterval time.Duration
	RetentionPeriod time.Duration
}

// DefaultConfig returns default processor configuration
func DefaultConfig() ProcessorConfig {
	return ProcessorConfig{
		BatchSize:       100,
		PollingInterval: 1 * time.Second,
		RetryDelay:      5 * time.Second,
		MaxRetries:      3,
		CleanupInterval: 1 * time.Hour,
		RetentionPeriod: 7 * 24 * time.Hour, // 7 days
	}
}

// Processor handles outbox message processing and publishing
type Processor struct {
	config    ProcessorConfig
	repo      *Repository
	publisher *publisher.Producer
	log       *logger.Logger
	tracer    trace.Tracer
}

// NewProcessor creates a new outbox processor
func NewProcessor(config ProcessorConfig, repo *Repository, pub *publisher.Producer, log *logger.Logger) *Processor {
	return &Processor{
		config:    config,
		repo:      repo,
		publisher: pub,
		log:       log,
		tracer:    otel.GetTracerProvider().Tracer("outbox-processor"),
	}
}

// Start begins processing outbox messages
func (p *Processor) Start(ctx context.Context) error {
	p.log.Info("Starting outbox processor",
		zap.Int("batch_size", p.config.BatchSize),
		zap.Duration("polling_interval", p.config.PollingInterval),
	)

	// Try processing a test batch to verify everything works
	if err := p.processBatch(ctx); err != nil {
		return fmt.Errorf("failed to process initial batch: %w", err)
	}

	// Start message processing loop
	go p.processMessages(ctx)

	// Start cleanup routine
	go p.runCleanup(ctx)

	return nil
}

func (p *Processor) processMessages(ctx context.Context) {
	ticker := time.NewTicker(p.config.PollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.processBatch(ctx); err != nil {
				p.log.Error("Failed to process message batch",
					zap.Error(err),
				)
			}
		}
	}
}

func (p *Processor) processBatch(ctx context.Context) error {
	ctx, span := p.tracer.Start(ctx, "outbox.process_batch")
	defer span.End()

	messages, err := p.repo.GetPendingMessages(ctx, p.config.BatchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	if len(messages) == 0 {
		return nil
	}

	span.SetAttributes(attribute.Int("batch.size", len(messages)))

	for _, msg := range messages {
		if err := p.processMessage(ctx, msg); err != nil {
			p.log.Error("Failed to process message",
				zap.String("message_id", msg.ID),
				zap.Error(err),
			)

			if msg.RetryCount >= p.config.MaxRetries {
				p.log.Error("Message exceeded max retries",
					zap.String("message_id", msg.ID),
					zap.Int("retry_count", msg.RetryCount),
				)
				if err := p.repo.MarkAsFailed(ctx, msg.ID, "Exceeded max retries"); err != nil {
					p.log.Error("Failed to mark message as failed",
						zap.String("message_id", msg.ID),
						zap.Error(err),
					)
				}
				continue
			}

			if err := p.repo.MarkAsFailed(ctx, msg.ID, err.Error()); err != nil {
				p.log.Error("Failed to mark message as failed",
					zap.String("message_id", msg.ID),
					zap.Error(err),
				)
			}
			continue
		}

		if err := p.repo.MarkAsPublished(ctx, msg.ID); err != nil {
			p.log.Error("Failed to mark message as published",
				zap.String("message_id", msg.ID),
				zap.Error(err),
			)
		}
	}

	return nil
}

func (p *Processor) processMessage(ctx context.Context, msg *Message) error {
	ctx, span := p.tracer.Start(ctx, "outbox.process_message",
		trace.WithAttributes(
			attribute.String("message.id", msg.ID),
			attribute.String("message.type", msg.EventType),
		),
	)
	defer span.End()

	// Publish message
	err := p.publisher.Publish(ctx, msg.Topic, msg.ID, msg.Payload)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	p.log.Debug("Successfully published message",
		zap.String("message_id", msg.ID),
		zap.String("topic", msg.Topic),
	)

	return nil
}

func (p *Processor) runCleanup(ctx context.Context) {
	ticker := time.NewTicker(p.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := p.repo.CleanupPublishedMessages(ctx, p.config.RetentionPeriod)
			if err != nil {
				p.log.Error("Failed to cleanup messages",
					zap.Error(err),
				)
				continue
			}

			if count > 0 {
				p.log.Info("Cleaned up old messages",
					zap.Int64("count", count),
				)
			}
		}
	}
}
