package command

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Handler defines the interface for command handlers
type Handler interface {
	HandleCommand(ctx context.Context, cmd *Command) error
	CanHandle(cmdType string) bool
}

// Processor manages command processing
type Processor struct {
	handlers   map[string][]Handler
	validator  CommandValidator
	log        *logger.Logger
	tracer     trace.Tracer
	workerPool chan struct{}
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// ProcessorConfig holds configuration for the command processor
type ProcessorConfig struct {
	MaxWorkers     int
	QueueSize      int
	DefaultTimeout time.Duration
}

// NewProcessor creates a new command processor
func NewProcessor(cfg ProcessorConfig, log *logger.Logger) *Processor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Processor{
		handlers:   make(map[string][]Handler),
		validator:  NewValidator(),
		log:        log,
		tracer:     otel.GetTracerProvider().Tracer("command-processor"),
		workerPool: make(chan struct{}, cfg.MaxWorkers),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// RegisterHandler registers a handler for specific command types
func (p *Processor) RegisterHandler(handler Handler) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for cmdType := range p.handlers {
		if handler.CanHandle(cmdType) {
			p.handlers[cmdType] = append(p.handlers[cmdType], handler)
		}
	}
}

// Process handles a command
func (p *Processor) Process(ctx context.Context, cmd *Command) error {
	ctx, span := p.tracer.Start(ctx, "process_command",
		trace.WithAttributes(
			attribute.String("command.id", cmd.ID),
			attribute.String("command.type", cmd.Type),
			attribute.Int("command.priority", int(cmd.Priority)),
		),
	)
	defer span.End()

	// Validate command
	if err := p.validator.ValidateCommand(ctx, cmd); err != nil {
		p.log.Error("Command validation failed",
			zap.String("command_id", cmd.ID),
			zap.String("command_type", cmd.Type),
			zap.Error(err),
		)
		cmd.SetError("VALIDATION_ERROR", err.Error(), "")
		cmd.Status = StatusFailed
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Acquire worker from pool
	select {
	case p.workerPool <- struct{}{}:
		defer func() { <-p.workerPool }()
	case <-ctx.Done():
		return ctx.Err()
	}

	// Update command status
	cmd.Status = StatusProcessing
	cmd.ProcessedAt = p.now()
	cmd.UpdatedAt = *cmd.ProcessedAt

	// Get handlers for command type
	p.mu.RLock()
	handlers := p.handlers[cmd.Type]
	p.mu.RUnlock()

	if len(handlers) == 0 {
		err := fmt.Errorf("no handlers registered for command type: %s", cmd.Type)
		cmd.SetError("NO_HANDLER", err.Error(), "")
		cmd.Status = StatusFailed
		return err
	}

	// Execute handlers
	var lastErr error
	for _, handler := range handlers {
		if err := handler.HandleCommand(ctx, cmd); err != nil {
			p.log.Error("Handler failed",
				zap.String("command_id", cmd.ID),
				zap.String("command_type", cmd.Type),
				zap.String("handler", fmt.Sprintf("%T", handler)),
				zap.Error(err),
			)
			lastErr = err
			cmd.SetError("HANDLER_ERROR", err.Error(), fmt.Sprintf("Handler: %T", handler))

			if cmd.IsRetryable() {
				cmd.Status = StatusRetrying
				cmd.RetryCount++
				cmd.ScheduledFor = p.calculateNextRetry(cmd)
			} else {
				cmd.Status = StatusFailed
			}
			break
		}
	}

	if lastErr == nil {
		cmd.Status = StatusCompleted
		cmd.CompletedAt = p.now()
		cmd.UpdatedAt = *cmd.CompletedAt
	}

	return lastErr
}

// Stop gracefully stops the processor
func (p *Processor) Stop() {
	p.cancel()
}

func (p *Processor) now() *time.Time {
	now := time.Now()
	return &now
}

func (p *Processor) calculateNextRetry(cmd *Command) *time.Time {
	backoff := cmd.RetryBackoff * time.Duration(cmd.RetryCount)
	next := time.Now().Add(backoff)
	return &next
}

// Status returns nil if the processor is healthy and running, or an error if there are issues
func (p *Processor) Status() error {
	select {
	case <-p.ctx.Done():
		return fmt.Errorf("processor is stopped")
	default:
		return nil
	}
}
