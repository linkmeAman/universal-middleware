package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.uber.org/zap"
)

// WarmupStrategy defines how to warm up the cache
type WarmupStrategy interface {
	// Warmup populates the cache with data
	Warmup(ctx context.Context) error
}

// KeyPattern represents a cache key pattern to warm up
type KeyPattern struct {
	Pattern string
	TTL     time.Duration
	Loader  func(ctx context.Context) (map[string]interface{}, error)
}

// CacheWarmer handles cache warm-up operations
type CacheWarmer struct {
	cache *RedisCache
	log   *logger.Logger
	mu    sync.RWMutex
	keys  []KeyPattern
}

// NewCacheWarmer creates a new cache warmer
func NewCacheWarmer(cache *RedisCache, log *logger.Logger) *CacheWarmer {
	return &CacheWarmer{
		cache: cache,
		log:   log,
		keys:  make([]KeyPattern, 0),
	}
}

// RegisterPattern adds a new key pattern to warm up
func (w *CacheWarmer) RegisterPattern(pattern KeyPattern) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.keys = append(w.keys, pattern)
}

// WarmupAll populates all registered patterns
func (w *CacheWarmer) WarmupAll(ctx context.Context) error {
	w.mu.RLock()
	patterns := make([]KeyPattern, len(w.keys))
	copy(patterns, w.keys)
	w.mu.RUnlock()

	var wg sync.WaitGroup
	errCh := make(chan error, len(patterns))

	for _, pattern := range patterns {
		wg.Add(1)
		go func(p KeyPattern) {
			defer wg.Done()
			if err := w.warmupPattern(ctx, p); err != nil {
				errCh <- fmt.Errorf("failed to warm up pattern %s: %w", p.Pattern, err)
			}
		}(pattern)
	}

	// Wait for all warm-ups to complete
	wg.Wait()
	close(errCh)

	// Collect any errors
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("warm-up errors: %v", errors)
	}
	return nil
}

func (w *CacheWarmer) warmupPattern(ctx context.Context, pattern KeyPattern) error {
	w.log.Info("Warming up cache pattern", zap.String("pattern", pattern.Pattern))

	// Load data using the provided loader
	data, err := pattern.Loader(ctx)
	if err != nil {
		return fmt.Errorf("failed to load data: %w", err)
	}

	// Cache each key-value pair
	for key, value := range data {
		// Store string values directly without JSON encoding
		if str, ok := value.(string); ok {
			if err := w.cache.Set(ctx, key, []byte(str), pattern.TTL); err != nil {
				w.log.Error("Failed to cache value",
					zap.String("key", key),
					zap.Error(err),
				)
				continue
			}
		} else {
			// For non-string values, use JSON encoding
			valueBytes, err := json.Marshal(value)
			if err != nil {
				w.log.Error("Failed to marshal value",
					zap.String("key", key),
					zap.Error(err),
				)
				continue
			}

			if err := w.cache.Set(ctx, key, valueBytes, pattern.TTL); err != nil {
				w.log.Error("Failed to cache value",
					zap.String("key", key),
					zap.Error(err),
				)
				continue
			}
		}
	}

	w.log.Info("Cache warm-up completed",
		zap.String("pattern", pattern.Pattern),
		zap.Int("keys", len(data)),
	)
	return nil
}
