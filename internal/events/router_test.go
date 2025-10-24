package events_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/linkmeAman/universal-middleware/internal/events"
	"github.com/linkmeAman/universal-middleware/internal/events/schemas"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockHandler is a test implementation of EventHandler
type MockHandler struct {
	mu           sync.Mutex
	handledCount int
	shouldError  bool
}

func (h *MockHandler) HandleEvent(ctx context.Context, event *schemas.Event) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handledCount++

	if h.shouldError {
		return errors.New("mock handler error")
	}
	return nil
}

func (h *MockHandler) GetHandledCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.handledCount
}

func TestRouter(t *testing.T) {
	log := logger.NewTestLogger()
	router := events.NewRouter(log)

	t.Run("successful event handling", func(t *testing.T) {
		handler1 := &MockHandler{}
		handler2 := &MockHandler{}

		eventType := schemas.EventTypeCommandReceived
		router.RegisterHandler(eventType, handler1)
		router.RegisterHandler(eventType, handler2)

		event := &schemas.Event{
			ID:   "test-event-1",
			Type: eventType,
			Time: time.Now(),
			Data: map[string]interface{}{
				"test": "data",
			},
		}

		err := router.HandleEvent(context.Background(), event)
		require.NoError(t, err)

		assert.Equal(t, 1, handler1.GetHandledCount())
		assert.Equal(t, 1, handler2.GetHandledCount())
	})

	t.Run("handler error propagation", func(t *testing.T) {
		handler := &MockHandler{shouldError: true}

		eventType := schemas.EventTypeCommandFailed
		router.RegisterHandler(eventType, handler)

		event := &schemas.Event{
			ID:   "test-event-2",
			Type: eventType,
			Time: time.Now(),
		}

		err := router.HandleEvent(context.Background(), event)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mock handler error")
	})

	t.Run("unregistered event type", func(t *testing.T) {
		event := &schemas.Event{
			ID:   "test-event-3",
			Type: "unknown.event",
			Time: time.Now(),
		}

		err := router.HandleEvent(context.Background(), event)
		require.NoError(t, err) // Should not error, just log warning
	})

	t.Run("handler unregistration", func(t *testing.T) {
		handler := &MockHandler{}
		eventType := schemas.EventTypeCommandCompleted

		router.RegisterHandler(eventType, handler)
		router.UnregisterHandler(eventType, handler)

		event := &schemas.Event{
			ID:   "test-event-4",
			Type: eventType,
			Time: time.Now(),
		}

		err := router.HandleEvent(context.Background(), event)
		require.NoError(t, err)
		assert.Equal(t, 0, handler.GetHandledCount())
	})
}

func TestRouterConcurrency(t *testing.T) {
	log := logger.NewTestLogger()
	router := events.NewRouter(log)
	handler := &MockHandler{}

	eventType := schemas.EventTypeCommandProcessed
	router.RegisterHandler(eventType, handler)

	// Test concurrent event handling
	var wg sync.WaitGroup
	eventCount := 100

	for i := 0; i < eventCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			event := &schemas.Event{
				ID:   fmt.Sprintf("concurrent-event-%d", index),
				Type: eventType,
				Time: time.Now(),
			}
			err := router.HandleEvent(context.Background(), event)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
	assert.Equal(t, eventCount, handler.GetHandledCount())
}
