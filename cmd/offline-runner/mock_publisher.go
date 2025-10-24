package main

import (
	"context"
	"fmt"
)

// MockPublisher is a lightweight publisher used for offline demos. It prints messages to stdout.
type MockPublisher struct{}

func NewMockPublisher() *MockPublisher { return &MockPublisher{} }

func (m *MockPublisher) Publish(ctx context.Context, topic string, key string, value []byte) error {
	fmt.Printf("[mock publish] topic=%s key=%s value=%s\n", topic, key, string(value))
	return nil
}

func (m *MockPublisher) PublishBatch(ctx context.Context, topic string, messages []struct {
	Key   string
	Value []byte
}) error {
	for _, msg := range messages {
		if err := m.Publish(ctx, topic, msg.Key, msg.Value); err != nil {
			return err
		}
	}
	return nil
}
