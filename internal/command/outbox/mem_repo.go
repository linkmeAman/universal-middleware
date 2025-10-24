package outbox

import (
	"context"
	"sync"
	"time"
)

// InMemoryRepository is a thread-safe in-memory implementation of the outbox repository.
// Useful for offline testing and demos.
type InMemoryRepository struct {
	mu       sync.Mutex
	messages map[string]*Message
	order    []string
}

// NewInMemoryRepository creates a new in-memory outbox repository
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		messages: make(map[string]*Message),
		order:    make([]string, 0),
	}
}

// Save stores a new message in the outbox
func (r *InMemoryRepository) Save(ctx context.Context, msg *Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	msg.Status = StatusPending
	r.messages[msg.ID] = msg
	r.order = append(r.order, msg.ID)
	return nil
}

// GetPendingMessages retrieves pending messages for processing
func (r *InMemoryRepository) GetPendingMessages(ctx context.Context, limit int) ([]*Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var out []*Message
	count := 0
	for _, id := range r.order {
		if count >= limit {
			break
		}
		m := r.messages[id]
		if m != nil && m.Status == StatusPending {
			out = append(out, m)
			count++
		}
	}
	return out, nil
}

// MarkAsPublished marks a message as successfully published
func (r *InMemoryRepository) MarkAsPublished(ctx context.Context, messageID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.messages[messageID]; ok {
		now := time.Now()
		m.Status = StatusPublished
		m.PublishedAt = &now
		return nil
	}
	return nil
}

// MarkAsFailed marks a message as failed with error details
func (r *InMemoryRepository) MarkAsFailed(ctx context.Context, messageID string, errorMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.messages[messageID]; ok {
		m.RetryCount++
		m.ErrorMessage = errorMsg
		m.Status = StatusFailed
		return nil
	}
	return nil
}

// CleanupPublishedMessages removes old published messages
func (r *InMemoryRepository) CleanupPublishedMessages(ctx context.Context, olderThan time.Duration) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-olderThan)
	var removed int64
	newOrder := make([]string, 0, len(r.order))
	for _, id := range r.order {
		m := r.messages[id]
		if m == nil {
			continue
		}
		if m.Status == StatusPublished && m.PublishedAt != nil && m.PublishedAt.Before(cutoff) {
			delete(r.messages, id)
			removed++
			continue
		}
		newOrder = append(newOrder, id)
	}
	r.order = newOrder
	return removed, nil
}
