package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/linkmeAman/universal-middleware/internal/cache"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.uber.org/zap"
)

// CachedRepository adds caching capabilities to any repository
type CachedRepository struct {
	cache      *cache.RedisCache
	logger     *logger.Logger
	keyPrefix  string
	defaultTTL time.Duration
}

// NewCachedRepository creates a new cached repository wrapper
func NewCachedRepository(cache *cache.RedisCache, log *logger.Logger, keyPrefix string, defaultTTL time.Duration) *CachedRepository {
	return &CachedRepository{
		cache:      cache,
		logger:     log,
		keyPrefix:  keyPrefix,
		defaultTTL: defaultTTL,
	}
}

// buildKey builds a cache key with the repository prefix
func (cr *CachedRepository) buildKey(key string) string {
	return fmt.Sprintf("%s:%s", cr.keyPrefix, key)
}

// Get retrieves an item from cache
func (cr *CachedRepository) Get(ctx context.Context, key string, value interface{}) error {
	cacheKey := cr.buildKey(key)
	data, _, err := cr.cache.Get(ctx, cacheKey)
	if err == nil {
		return json.Unmarshal(data, value)
	}
	return err
}

// Set stores an item in cache
func (cr *CachedRepository) Set(ctx context.Context, key string, value interface{}, ttl ...time.Duration) error {
	cacheKey := cr.buildKey(key)
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// Use provided TTL or default
	expiry := cr.defaultTTL
	if len(ttl) > 0 {
		expiry = ttl[0]
	}

	return cr.cache.Set(ctx, cacheKey, data, expiry)
}

// Delete removes an item from cache
func (cr *CachedRepository) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	cacheKeys := make([]string, len(keys))
	for i, key := range keys {
		cacheKeys[i] = cr.buildKey(key)
	}

	err := cr.cache.InvalidateByPattern(ctx, cr.buildKey("*"))
	if err != nil {
		cr.logger.Error("Failed to delete cache keys",
			zap.Strings("keys", cacheKeys),
			zap.Error(err),
		)
	}
	return err
}

// GetOrSet gets a value from cache or sets it if not found
func (cr *CachedRepository) GetOrSet(
	ctx context.Context,
	key string,
	value interface{},
	fetch func(context.Context) (interface{}, error),
) error {
	// Try to get from cache first
	if err := cr.Get(ctx, key, value); err == nil {
		return nil
	}

	// Fetch data if not in cache
	data, err := fetch(ctx)
	if err != nil {
		return err
	}

	// Store in cache and return
	if err := cr.Set(ctx, key, data); err != nil {
		cr.logger.Error("Failed to cache value",
			zap.String("key", key),
			zap.Error(err),
		)
	}

	// Copy fetched data to the output value
	if valueBytes, err := json.Marshal(data); err == nil {
		return json.Unmarshal(valueBytes, value)
	}

	return nil
}

// InvalidatePattern invalidates all cache entries matching a pattern
func (cr *CachedRepository) InvalidatePattern(ctx context.Context, pattern string) error {
	return cr.cache.InvalidateByPattern(ctx, cr.buildKey(pattern))
}

// WarmUp pre-populates the cache with data
func (cr *CachedRepository) WarmUp(ctx context.Context, patterns []cache.KeyPattern) error {
	warmer := cache.NewCacheWarmer(cr.cache, cr.logger)

	// Register patterns with prefixed keys
	for _, pattern := range patterns {
		pattern.Pattern = cr.buildKey(pattern.Pattern)
		warmer.RegisterPattern(pattern)
	}

	return warmer.WarmupAll(ctx)
}
