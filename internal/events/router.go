package events

import (
	"context"
	"fmt"
	"sync"

	"github.com/linkmeAman/universal-middleware/internal/events/schemas"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// EventHandler defines the interface for event handlers
type EventHandler interface {
	HandleEvent(ctx context.Context, event *schemas.Event) error
}

// Router routes events to appropriate handlers
type Router struct {
	handlers map[schemas.EventType][]EventHandler
	mu       sync.RWMutex
	log      *logger.Logger
	tracer   trace.Tracer
}

// NewRouter creates a new event router
func NewRouter(log *logger.Logger) *Router {
	return &Router{
		handlers: make(map[schemas.EventType][]EventHandler),
		log:      log,
		tracer:   otel.GetTracerProvider().Tracer("event-router"),
	}
}

// RegisterHandler registers a handler for an event type
func (r *Router) RegisterHandler(eventType schemas.EventType, handler EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers[eventType] = append(r.handlers[eventType], handler)
	r.log.Info("Registered event handler",
		zap.String("event_type", string(eventType)),
		zap.String("handler", fmt.Sprintf("%T", handler)),
	)
}

// HandleEvent routes an event to all registered handlers
func (r *Router) HandleEvent(ctx context.Context, event *schemas.Event) error {
	ctx, span := r.tracer.Start(ctx, "router.handle_event",
		trace.WithAttributes(
			attribute.String("event.type", string(event.Type)),
			attribute.String("event.id", event.ID),
		),
	)
	defer span.End()

	r.mu.RLock()
	handlers, exists := r.handlers[event.Type]
	r.mu.RUnlock()

	if !exists || len(handlers) == 0 {
		r.log.Warn("No handlers registered for event type",
			zap.String("event_type", string(event.Type)),
			zap.String("event_id", event.ID),
		)
		return nil
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()

			handlerSpan := trace.SpanFromContext(ctx)
			handlerCtx := trace.ContextWithSpan(ctx, handlerSpan)

			if err := h.HandleEvent(handlerCtx, event); err != nil {
				r.log.Error("Handler failed to process event",
					zap.String("event_type", string(event.Type)),
					zap.String("event_id", event.ID),
					zap.String("handler", fmt.Sprintf("%T", h)),
					zap.Error(err),
				)
				errCh <- fmt.Errorf("handler %T failed: %w", h, err)
			}
		}(handler)
	}

	// Wait for all handlers to complete
	wg.Wait()
	close(errCh)

	// Collect all errors
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("event handling failed with %d errors: %v", len(errors), errors)
	}

	r.log.Debug("Event processed successfully",
		zap.String("event_type", string(event.Type)),
		zap.String("event_id", event.ID),
		zap.Int("handlers_count", len(handlers)),
	)

	return nil
}

// UnregisterHandler removes a handler for an event type
func (r *Router) UnregisterHandler(eventType schemas.EventType, handler EventHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	handlers := r.handlers[eventType]
	for i, h := range handlers {
		if h == handler {
			r.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			r.log.Info("Unregistered event handler",
				zap.String("event_type", string(eventType)),
				zap.String("handler", fmt.Sprintf("%T", handler)),
			)
			break
		}
	}
}
