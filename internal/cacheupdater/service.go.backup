package cacheupdater

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/linkmeAman/universal-middleware/internal/cache"
	"github.com/linkmeAman/universal-middleware/internal/events/consumer"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
	"go.uber.org/zap"
)

// Service handles cache invalidation based on events
type Service struct {
	consumer *consumer.Consumer
	cache    *cache.RedisCache
	log      *logger.Logger
}

// ConsumerStatus represents the status of the Kafka consumer
type ConsumerStatus struct {
	Connected bool
	Error     string
}

// GetConsumerStatus returns the current status of the Kafka consumer
func (s *Service) GetConsumerStatus() ConsumerStatus {
	if s.consumer == nil {
		return ConsumerStatus{
			Connected: false,
			Error:     "consumer not initialized",
		}
	}

	if err := s.consumer.Ping(); err != nil {
		return ConsumerStatus{
			Connected: false,
			Error:     err.Error(),
		}
	}

	return ConsumerStatus{
		Connected: true,
	}
}

// GetRedisClient returns the Redis client for health checks
func (s *Service) GetRedisClient() *cache.RedisCache {
	return s.cache
}

// NewService creates a new cache updater service
func NewService(redisURL string, kafkaBrokers []string, topic string, log *logger.Logger) (*Service, error) {
	// Initialize metrics
	m := metrics.New("cache_updater")

	// Create Redis client
	cacheClient := cache.NewRedisCache(cache.CacheOptions{
		Addresses:   []string{redisURL},
		PoolSize:    10,
		BaseTTL:     time.Hour,
		NegativeTTL: 5 * time.Minute,
	}, log, m)

	// Create event consumer
	consumer, err := consumer.NewConsumer(consumer.ConsumerConfig{
		Brokers:          kafkaBrokers,
		Topics:           []string{topic},
		GroupID:          "cache-updater",
		InitialOffset:    sarama.OffsetNewest,
		SessionTimeout:   30 * time.Second,
		RebalanceTimeout: 30 * time.Second,
		MinBytes:         1,       // Minimum bytes to fetch
		MaxBytes:         1048576, // Maximum bytes to fetch (1MB)
		MaxWait:          5 * time.Second,
	}, nil, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	svc := &Service{
		consumer: consumer,
		cache:    cacheClient,
		log:      log,
	}

	return svc, nil
}

// Start begins processing events
func (s *Service) Start() error {
	return s.consumer.Start()
}

// Stop gracefully shuts down the service
func (s *Service) Stop() error {
	return s.consumer.Stop()
}

// Handle implements the consumer.Handler interface
func (s *Service) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var evt struct {
		Type    string `json:"type"`
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Invalidate cache based on event type
	switch evt.Type {
	case "entity.updated", "cache.invalidate":
		if err := s.cache.InvalidateByPattern(ctx, evt.Pattern); err != nil {
			return fmt.Errorf("failed to invalidate cache: %w", err)
		}
		s.log.Info("Cache invalidated",
			zap.String("pattern", evt.Pattern),
			zap.String("type", evt.Type),
		)
	}

	return nil
}
