package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/linkmeAman/universal-middleware/internal/database"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Status represents the outbox message status
type Status string

const (
	StatusPending   Status = "pending"
	StatusPublished Status = "published"
	StatusFailed    Status = "failed"
)

// Message represents an outbox message
type Message struct {
	ID            string          `json:"id"`
	AggregateType string          `json:"aggregateType"`
	AggregateID   string          `json:"aggregateId"`
	EventType     string          `json:"eventType"`
	Payload       json.RawMessage `json:"payload"`
	Topic         string          `json:"topic"`
	Status        Status          `json:"status"`
	CreatedAt     time.Time       `json:"createdAt"`
	PublishedAt   *time.Time      `json:"publishedAt,omitempty"`
	RetryCount    int             `json:"retryCount"`
	ErrorMessage  string          `json:"errorMessage,omitempty"`
}

// Repository handles outbox message persistence
type Repository struct {
	db     database.DB
	log    *logger.Logger
	tracer trace.Tracer
}

// NewRepository creates a new outbox repository
func NewRepository(db database.DB, log *logger.Logger) *Repository {
	return &Repository{
		db:     db,
		log:    log,
		tracer: otel.GetTracerProvider().Tracer("outbox-repository"),
	}
}

// Save stores a new message in the outbox
func (r *Repository) Save(ctx context.Context, msg *Message) error {
	ctx, span := r.tracer.Start(ctx, "outbox.save",
		trace.WithAttributes(
			attribute.String("message.id", msg.ID),
			attribute.String("message.type", msg.EventType),
		),
	)
	defer span.End()

	query := `
		INSERT INTO outbox_messages (
			id, aggregate_type, aggregate_id, event_type, 
			payload, topic, status, created_at, 
			retry_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.Exec(ctx, query,
		msg.ID, msg.AggregateType, msg.AggregateID, msg.EventType,
		msg.Payload, msg.Topic, msg.Status, msg.CreatedAt,
		msg.RetryCount,
	)

	if err != nil {
		r.log.Error("Failed to save outbox message",
			zap.String("message_id", msg.ID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to save outbox message: %w", err)
	}

	return nil
}

// GetPendingMessages retrieves pending messages for processing
func (r *Repository) GetPendingMessages(ctx context.Context, limit int) ([]*Message, error) {
	ctx, span := r.tracer.Start(ctx, "outbox.get_pending",
		trace.WithAttributes(
			attribute.Int("limit", limit),
		),
	)
	defer span.End()

	query := `
		SELECT id, aggregate_type, aggregate_id, event_type,
			   payload, topic, status, created_at, published_at,
			   retry_count, error_message
		FROM outbox_messages
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED`

	rows, err := r.db.Query(ctx, query, StatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending messages: %w", err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		msg := &Message{}
		err := rows.Scan(
			&msg.ID, &msg.AggregateType, &msg.AggregateID, &msg.EventType,
			&msg.Payload, &msg.Topic, &msg.Status, &msg.CreatedAt, &msg.PublishedAt,
			&msg.RetryCount, &msg.ErrorMessage,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

// MarkAsPublished marks a message as successfully published
func (r *Repository) MarkAsPublished(ctx context.Context, messageID string) error {
	ctx, span := r.tracer.Start(ctx, "outbox.mark_published",
		trace.WithAttributes(
			attribute.String("message.id", messageID),
		),
	)
	defer span.End()

	now := time.Now()
	query := `
		UPDATE outbox_messages 
		SET status = $1, published_at = $2
		WHERE id = $3`

	result, err := r.db.Exec(ctx, query, StatusPublished, now, messageID)
	if err != nil {
		return fmt.Errorf("failed to mark message as published: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("no message found with ID: %s", messageID)
	}

	return nil
}

// MarkAsFailed marks a message as failed with error details
func (r *Repository) MarkAsFailed(ctx context.Context, messageID string, errorMsg string) error {
	ctx, span := r.tracer.Start(ctx, "outbox.mark_failed",
		trace.WithAttributes(
			attribute.String("message.id", messageID),
			attribute.String("error", errorMsg),
		),
	)
	defer span.End()

	query := `
		UPDATE outbox_messages 
		SET status = $1, error_message = $2, retry_count = retry_count + 1
		WHERE id = $3`

	result, err := r.db.Exec(ctx, query, StatusFailed, errorMsg, messageID)
	if err != nil {
		return fmt.Errorf("failed to mark message as failed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("no message found with ID: %s", messageID)
	}

	return nil
}

// CleanupPublishedMessages removes old published messages
func (r *Repository) CleanupPublishedMessages(ctx context.Context, olderThan time.Duration) (int64, error) {
	ctx, span := r.tracer.Start(ctx, "outbox.cleanup",
		trace.WithAttributes(
			attribute.String("cleanup.duration", olderThan.String()),
		),
	)
	defer span.End()

	cutoff := time.Now().Add(-olderThan)
	query := `
		DELETE FROM outbox_messages 
		WHERE status = $1 
		AND published_at < $2`

	result, err := r.db.Exec(ctx, query, StatusPublished, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup messages: %w", err)
	}

	rowCount := result.RowsAffected()
	r.log.Info("Cleaned up published messages",
		zap.Int64("deleted_count", rowCount),
		zap.Time("cutoff_time", cutoff),
	)

	return rowCount, nil
}

// CreateMessage creates a new outbox message
func CreateMessage(aggregateType, aggregateID, eventType string, payload interface{}) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	return &Message{
		ID:            uuid.New().String(),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payloadBytes,
		Status:        StatusPending,
		CreatedAt:     time.Now(),
		RetryCount:    0,
	}, nil
}
