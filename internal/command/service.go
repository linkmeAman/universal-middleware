package command

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CommandService handles async write operations with Redis buffering
type CommandService struct {
	db          *sql.DB
	redisClient *redis.Client
	outbox      *OutboxProcessor
	log         *zap.Logger
}

// Command represents a write operation
// type Command struct {
// 	ID             string                 `json:"id"`
// 	Type           string                 `json:"type"`
// 	EntityID       string                 `json:"entity_id"`
// 	Payload        map[string]interface{} `json:"payload"`
// 	IdempotencyKey string                 `json:"idempotency_key"`
// 	Status         string                 `json:"status"`
// 	CreatedAt      time.Time              `json:"created_at"`
// 	ProcessedAt    *time.Time             `json:"processed_at"`
// 	Error          string                 `json:"error,omitempty"`
// }

// CommandResult represents the result of command submission
type CommandResult struct {
	CommandID string `json:"command_id"`
	Status    string `json:"status"`
	Location  string `json:"location"`
}

// NewCommandService creates a new command service
func NewCommandService(
	db *sql.DB,
	redisAddr string,
	log *zap.Logger,
) (*CommandService, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       1, // Use different DB for command tracking
	})

	svc := &CommandService{
		db:          db,
		redisClient: rdb,
		log:         log,
	}

	// Initialize outbox processor
	outbox, err := NewOutboxProcessor(db, log)
	if err != nil {
		return nil, err
	}
	svc.outbox = outbox

	return svc, nil
}

// SubmitCommand accepts a write command and returns immediately
func (s *CommandService) SubmitCommand(ctx context.Context, cmd *Command) (*CommandResult, error) {
	// Check idempotency
	if cmd.IdempotencyKey != "" {
		if existing, err := s.checkIdempotency(ctx, cmd.IdempotencyKey); err == nil {
			s.log.Info("Idempotent request detected",
				zap.String("command_id", existing.ID),
				zap.String("idempotency_key", cmd.IdempotencyKey))
			return &CommandResult{
				CommandID: existing.ID,
				Status:    string(existing.Status),
				Location:  fmt.Sprintf("/api/v1/commands/%s", existing.ID),
			}, nil
		}
	}

	// Generate command ID
	cmd.ID = uuid.New().String()
	cmd.Status = "pending"
	cmd.CreatedAt = time.Now()

	// Start transaction
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Store command in outbox (transactional)
	if err := s.storeInOutbox(ctx, tx, cmd); err != nil {
		return nil, fmt.Errorf("failed to store in outbox: %w", err)
	}

	// 2. Store idempotency key (if provided)
	if cmd.IdempotencyKey != "" {
		if err := s.storeIdempotencyKey(ctx, tx, cmd); err != nil {
			return nil, fmt.Errorf("failed to store idempotency key: %w", err)
		}
	}

	// 3. Cache command status in Redis for fast status checks
	if err := s.cacheCommandStatus(ctx, cmd); err != nil {
		// Log but don't fail - DB is source of truth
		s.log.Warn("Failed to cache command status", zap.Error(err))
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Info("Command submitted",
		zap.String("command_id", cmd.ID),
		zap.String("type", cmd.Type))

	return &CommandResult{
		CommandID: cmd.ID,
		Status:    "accepted",
		Location:  fmt.Sprintf("/api/v1/commands/%s", cmd.ID),
	}, nil
}

// GetCommandStatus retrieves command status (cache-first)
func (s *CommandService) GetCommandStatus(ctx context.Context, commandID string) (*Command, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("cmd:status:%s", commandID)
	if cached, err := s.redisClient.Get(ctx, cacheKey).Result(); err == nil {
		var cmd Command
		if err := json.Unmarshal([]byte(cached), &cmd); err == nil {
			return &cmd, nil
		}
	}

	// Cache miss - query database
	var cmd Command
	query := `
		SELECT id, type, entity_id, payload, status, created_at, processed_at, error
		FROM commands
		WHERE id = $1
	`

	var payloadJSON []byte
	var processedAt sql.NullTime
	var errorMsg sql.NullString

	err := s.db.QueryRowContext(ctx, query, commandID).Scan(
		&cmd.ID,
		&cmd.Type,
		&cmd.EntityID,
		&payloadJSON,
		&cmd.Status,
		&cmd.CreatedAt,
		&processedAt,
		&errorMsg,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("command not found")
	}
	if err != nil {
		return nil, err
	}

	// Parse JSON payload
	json.Unmarshal(payloadJSON, &cmd.Payload)

	if processedAt.Valid {
		cmd.ProcessedAt = &processedAt.Time
	}
	if errorMsg.Valid {
		cmd.Error = errorMsg.String
	}

	// Update cache
	if data, err := json.Marshal(cmd); err == nil {
		ttl := 300 * time.Second
		if cmd.Status == "completed" || cmd.Status == "failed" {
			ttl = 3600 * time.Second // Cache completed commands longer
		}
		s.redisClient.Set(ctx, cacheKey, data, ttl)
	}

	return &cmd, nil
}

// storeInOutbox saves command to outbox table for processing
func (s *CommandService) storeInOutbox(ctx context.Context, tx *sql.Tx, cmd *Command) error {
	payloadJSON, err := json.Marshal(cmd.Payload)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO outbox_messages (
			id, aggregate_type, aggregate_id, event_type,
			payload, topic, status, created_at, retry_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 0)
	`

	_, err = tx.ExecContext(ctx, query,
		cmd.ID,
		"command",
		cmd.EntityID,
		cmd.Type,
		payloadJSON,
		"entity.commands",
		"pending",
		cmd.CreatedAt,
	)

	return err
}

// storeIdempotencyKey saves idempotency key to prevent duplicates
func (s *CommandService) storeIdempotencyKey(ctx context.Context, tx *sql.Tx, cmd *Command) error {
	query := `
		INSERT INTO idempotency_keys (key, command_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (key) DO NOTHING
	`

	_, err := tx.ExecContext(ctx, query, cmd.IdempotencyKey, cmd.ID, cmd.CreatedAt)
	return err
}

// checkIdempotency checks if a command with this idempotency key exists
func (s *CommandService) checkIdempotency(ctx context.Context, key string) (*Command, error) {
	query := `
		SELECT c.id, c.type, c.entity_id, c.status, c.created_at
		FROM commands c
		JOIN idempotency_keys i ON i.command_id = c.id
		WHERE i.key = $1
	`

	var cmd Command
	err := s.db.QueryRowContext(ctx, query, key).Scan(
		&cmd.ID,
		&cmd.Type,
		&cmd.EntityID,
		&cmd.Status,
		&cmd.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("not found")
	}

	return &cmd, err
}

// cacheCommandStatus stores command status in Redis for fast lookups
func (s *CommandService) cacheCommandStatus(ctx context.Context, cmd *Command) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("cmd:status:%s", cmd.ID)
	return s.redisClient.Set(ctx, key, data, 300*time.Second).Err()
}

// OutboxProcessor processes commands from the outbox
type OutboxProcessor struct {
	db        *sql.DB
	log       *zap.Logger
	batchSize int
}

// NewOutboxProcessor creates an outbox processor
func NewOutboxProcessor(db *sql.DB, log *zap.Logger) (*OutboxProcessor, error) {
	return &OutboxProcessor{
		db:        db,
		log:       log,
		batchSize: 100,
	}, nil
}

// Start begins processing the outbox
func (o *OutboxProcessor) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := o.processBatch(ctx); err != nil {
				o.log.Error("Failed to process outbox batch", zap.Error(err))
			}
		}
	}
}

// processBatch processes a batch of outbox messages
func (o *OutboxProcessor) processBatch(ctx context.Context) error {
	// Fetch pending messages with FOR UPDATE SKIP LOCKED
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, topic
		FROM outbox_messages
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`

	rows, err := o.db.QueryContext(ctx, query, o.batchSize)
	if err != nil {
		return err
	}
	defer rows.Close()

	messages := []OutboxMessage{}
	for rows.Next() {
		var msg OutboxMessage
		if err := rows.Scan(
			&msg.ID,
			&msg.AggregateType,
			&msg.AggregateID,
			&msg.EventType,
			&msg.Payload,
			&msg.Topic,
		); err != nil {
			return err
		}
		messages = append(messages, msg)
	}

	// Process each message
	for _, msg := range messages {
		if err := o.processMessage(ctx, msg); err != nil {
			o.log.Error("Failed to process message",
				zap.String("id", msg.ID),
				zap.Error(err))

			// Mark as failed after retries
			o.markFailed(ctx, msg.ID, err)
		} else {
			// Mark as published
			o.markPublished(ctx, msg.ID)
		}
	}

	return nil
}

// processMessage publishes a message to Kafka
func (o *OutboxProcessor) processMessage(ctx context.Context, msg OutboxMessage) error {
	// TODO: Implement Kafka producer
	// producer.SendMessage(msg.Topic, msg.ID, msg.Payload)

	o.log.Debug("Processing outbox message",
		zap.String("id", msg.ID),
		zap.String("type", msg.EventType))

	return nil
}

// markPublished marks a message as successfully published
func (o *OutboxProcessor) markPublished(ctx context.Context, messageID string) error {
	query := `
		UPDATE outbox_messages
		SET status = 'published', published_at = NOW()
		WHERE id = $1
	`
	_, err := o.db.ExecContext(ctx, query, messageID)
	return err
}

// markFailed marks a message as failed
func (o *OutboxProcessor) markFailed(ctx context.Context, messageID string, err error) error {
	query := `
		UPDATE outbox_messages
		SET status = 'failed', error_message = $2, retry_count = retry_count + 1
		WHERE id = $1
	`
	_, dbErr := o.db.ExecContext(ctx, query, messageID, err.Error())
	return dbErr
}

// OutboxMessage represents a message in the outbox
type OutboxMessage struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	Topic         string
}
