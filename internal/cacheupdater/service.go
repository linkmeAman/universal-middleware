package cacheupdater

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// CacheUpdateService handles cache invalidation and real-time updates
type CacheUpdateService struct {
	redisClient *redis.Client
	consumer    *EventConsumer
	wsHub       WebSocketPublisher
	log         *zap.Logger

	// Metrics
	cacheHits   int64
	cacheMisses int64
	updates     int64
}

// WebSocketPublisher interface for real-time updates
type WebSocketPublisher interface {
	PublishUpdate(ctx context.Context, room string, msg RealtimeMessage) error
}

// RealtimeMessage represents an update to send via WebSocket
type RealtimeMessage struct {
	Type      string          `json:"type"`
	Entity    string          `json:"entity"`
	Action    string          `json:"action"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

// Event represents a domain event from Kafka
type Event struct {
	Type       string                 `json:"type"`
	EntityID   string                 `json:"entity_id"`
	EntityType string                 `json:"entity_type"`
	Action     string                 `json:"action"`
	Data       map[string]interface{} `json:"data"`
	Metadata   map[string]interface{} `json:"metadata"`
	Timestamp  int64                  `json:"timestamp"`
}

// NewCacheUpdateService creates a new cache updater
func NewCacheUpdateService(
	redisAddr string,
	kafkaBrokers []string,
	wsHub WebSocketPublisher,
	log *zap.Logger,
) (*CacheUpdateService, error) {
	// Initialize Redis with retry logic
	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     "",
		DB:           0,
		PoolSize:     100,
		MinIdleConns: 10,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test connection with exponential backoff
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := rdb.Ping(ctx).Err(); err == nil {
			break
		}
		if i == 4 {
			return nil, fmt.Errorf("failed to connect to Redis after retries")
		}
		time.Sleep(time.Duration(1<<uint(i)) * time.Second)
	}

	svc := &CacheUpdateService{
		redisClient: rdb,
		wsHub:       wsHub,
		log:         log,
	}

	// Initialize Kafka consumer
	consumer, err := NewEventConsumer(kafkaBrokers, "cache-updater", svc.handleEvent, log)
	if err != nil {
		return nil, err
	}
	svc.consumer = consumer

	return svc, nil
}

// Start begins consuming events and updating cache
func (s *CacheUpdateService) Start(ctx context.Context) error {
	s.log.Info("Starting cache update service")
	return s.consumer.Start(ctx)
}

// handleEvent processes incoming events and updates cache + notifies clients
func (s *CacheUpdateService) handleEvent(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var event Event
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		s.log.Error("Failed to unmarshal event", zap.Error(err))
		return err // Will retry
	}

	s.log.Debug("Processing event",
		zap.String("type", event.Type),
		zap.String("entity_id", event.EntityID),
		zap.String("action", event.Action))

	// Process based on event type
	switch event.Type {
	case "entity.created":
		return s.handleEntityCreated(ctx, event)
	case "entity.updated":
		return s.handleEntityUpdated(ctx, event)
	case "entity.deleted":
		return s.handleEntityDeleted(ctx, event)
	default:
		s.log.Warn("Unknown event type", zap.String("type", event.Type))
		return nil // Ack and move on
	}
}

// handleEntityCreated warms cache and notifies clients
func (s *CacheUpdateService) handleEntityCreated(ctx context.Context, event Event) error {
	// Build cache key
	cacheKey := fmt.Sprintf("entity:%s", event.EntityID)

	// Serialize data
	data, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	// Set in cache with TTL (with jitter to prevent thundering herd)
	ttl := s.calculateTTL(300 * time.Second) // Base 5 minutes
	if err := s.redisClient.Set(ctx, cacheKey, data, ttl).Err(); err != nil {
		s.log.Error("Failed to set cache", zap.Error(err))
		// Continue to send WebSocket update even if cache fails
	}

	// Send real-time update
	return s.sendRealtimeUpdate(ctx, event, "created")
}

// handleEntityUpdated invalidates cache and notifies clients
func (s *CacheUpdateService) handleEntityUpdated(ctx context.Context, event Event) error {
	cacheKey := fmt.Sprintf("entity:%s", event.EntityID)

	// Strategy: Delete from cache (next read will fetch fresh data)
	// Alternative: Update in place if you have the full data
	if event.Data != nil && len(event.Data) > 0 {
		// We have the updated data - warm the cache
		data, err := json.Marshal(event.Data)
		if err != nil {
			return err
		}

		ttl := s.calculateTTL(300 * time.Second)
		s.redisClient.Set(ctx, cacheKey, data, ttl)
	} else {
		// Just invalidate - let next read repopulate
		s.redisClient.Del(ctx, cacheKey)
	}

	// Also invalidate related cache entries (if applicable)
	s.invalidateRelatedCache(ctx, event)

	// Send real-time update
	s.updates++
	return s.sendRealtimeUpdate(ctx, event, "updated")
}

// handleEntityDeleted removes from cache and notifies clients
func (s *CacheUpdateService) handleEntityDeleted(ctx context.Context, event Event) error {
	cacheKey := fmt.Sprintf("entity:%s", event.EntityID)

	// Delete from cache
	if err := s.redisClient.Del(ctx, cacheKey).Err(); err != nil {
		s.log.Error("Failed to delete from cache", zap.Error(err))
	}

	// Invalidate related cache
	s.invalidateRelatedCache(ctx, event)

	// Send real-time update
	return s.sendRealtimeUpdate(ctx, event, "deleted")
}

// sendRealtimeUpdate publishes update to WebSocket clients
func (s *CacheUpdateService) sendRealtimeUpdate(ctx context.Context, event Event, action string) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return err
	}

	msg := RealtimeMessage{
		Type:      event.EntityType,
		Entity:    event.EntityID,
		Action:    action,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	// Publish to specific entity room
	room := fmt.Sprintf("%s.%s", event.EntityType, event.EntityID)
	if err := s.wsHub.PublishUpdate(ctx, room, msg); err != nil {
		s.log.Error("Failed to publish WebSocket update",
			zap.Error(err),
			zap.String("room", room))
		// Don't return error - cache update succeeded
	}

	// Also publish to wildcard room (all entities of this type)
	wildcardRoom := fmt.Sprintf("%s.*", event.EntityType)
	s.wsHub.PublishUpdate(ctx, wildcardRoom, msg)

	return nil
}

// invalidateRelatedCache invalidates cache entries related to this entity
func (s *CacheUpdateService) invalidateRelatedCache(ctx context.Context, event Event) {
	// Example: If this entity is part of a list, invalidate list cache

	// Pattern-based invalidation (use with caution - can be expensive)
	if tags, ok := event.Metadata["cache_tags"].([]interface{}); ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				pattern := fmt.Sprintf("list:%s:*", tagStr)
				s.invalidateByPattern(ctx, pattern)
			}
		}
	}
}

// invalidateByPattern deletes keys matching a pattern (use sparingly)
func (s *CacheUpdateService) invalidateByPattern(ctx context.Context, pattern string) {
	iter := s.redisClient.Scan(ctx, 0, pattern, 100).Iterator()

	keys := []string{}
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())

		// Batch delete every 100 keys
		if len(keys) >= 100 {
			s.redisClient.Del(ctx, keys...)
			keys = keys[:0]
		}
	}

	// Delete remaining keys
	if len(keys) > 0 {
		s.redisClient.Del(ctx, keys...)
	}

	if err := iter.Err(); err != nil {
		s.log.Error("Failed to scan keys", zap.Error(err))
	}
}

// calculateTTL adds jitter to TTL to prevent thundering herd
func (s *CacheUpdateService) calculateTTL(baseTTL time.Duration) time.Duration {
	// Add ±10% jitter
	jitterRange := int64(float64(baseTTL) * 0.1)
	jitter := time.Duration((time.Now().UnixNano() % jitterRange) - jitterRange/2)
	return baseTTL + jitter
}

// GetMetrics returns service metrics
func (s *CacheUpdateService) GetMetrics() map[string]int64 {
	return map[string]int64{
		"cache_hits":   s.cacheHits,
		"cache_misses": s.cacheMisses,
		"updates":      s.updates,
	}
}

// GetRedisClient returns the Redis client for health checks
func (s *CacheUpdateService) GetRedisClient() *redis.Client {
	return s.redisClient
}

// GetConsumerStatus returns the Kafka consumer status
func (s *CacheUpdateService) GetConsumerStatus() struct {
	Connected bool
	Error     string
} {
	if s.consumer == nil || s.consumer.consumer == nil {
		return struct {
			Connected bool
			Error     string
		}{
			Connected: false,
			Error:     "consumer not initialized",
		}
	}
	return struct {
		Connected bool
		Error     string
	}{
		Connected: true,
		Error:     "",
	}
}

// Stop gracefully stops the service
func (s *CacheUpdateService) Stop() error {
	if s.consumer != nil && s.consumer.consumer != nil {
		return s.consumer.consumer.Close()
	}
	return nil
}

// NewService is a wrapper around NewCacheUpdateService for backward compatibility
func NewService(redisAddr string, kafkaBrokers []string, topic string, log *zap.Logger) (*CacheUpdateService, error) {
	// For now, we're not using the WebSocket hub in the cache updater
	return NewCacheUpdateService(redisAddr, kafkaBrokers, nil, log)
}

// EventConsumer wraps Kafka consumer with retry logic
type EventConsumer struct {
	consumer sarama.ConsumerGroup
	handler  func(context.Context, *sarama.ConsumerMessage) error
	log      *zap.Logger
}

// NewEventConsumer creates a Kafka consumer with retries
func NewEventConsumer(
	brokers []string,
	groupID string,
	handler func(context.Context, *sarama.ConsumerMessage) error,
	log *zap.Logger,
) (*EventConsumer, error) {
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin

	group, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, err
	}

	return &EventConsumer{
		consumer: group,
		handler:  handler,
		log:      log,
	}, nil
}

// Start begins consuming events
func (c *EventConsumer) Start(ctx context.Context) error {
	topics := []string{"entity.events"}

	go func() {
		for {
			if err := c.consumer.Consume(ctx, topics, c); err != nil {
				c.log.Error("Consumer error", zap.Error(err))
			}

			if ctx.Err() != nil {
				return
			}
		}
	}()

	return nil
}

// Setup is run at the beginning of a new session
func (c *EventConsumer) Setup(sarama.ConsumerGroupSession) error { return nil }

// Cleanup is run at the end of a session
func (c *EventConsumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

// ConsumeClaim processes messages with retry logic
func (c *EventConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		// Process with exponential backoff retry
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			if err = c.handler(session.Context(), msg); err == nil {
				session.MarkMessage(msg, "")
				break
			}

			c.log.Warn("Failed to process message, retrying",
				zap.Int("attempt", attempt+1),
				zap.Error(err))

			// Exponential backoff
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
		}

		if err != nil {
			c.log.Error("Failed to process message after retries",
				zap.Error(err),
				zap.String("topic", msg.Topic),
				zap.Int64("offset", msg.Offset))
			// Mark as processed to avoid blocking - send to DLQ in production
			session.MarkMessage(msg, "")
		}
	}

	return nil
}
