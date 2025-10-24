package cache

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

var (
	ErrCacheMiss = errors.New("cache miss")
	ErrNotFound  = errors.New("not found")
)

type CacheMeta struct {
	Status string // "hit", "miss", "neg"
}

// RedisCache implements cache with single-flight and jitter
type RedisCache struct {
	client      redis.UniversalClient
	sf          singleflight.Group
	baseTTL     time.Duration
	negativeTTL time.Duration
	logger      *logger.Logger
	metrics     *metrics.Metrics
	tracer      trace.Tracer
}

type CacheOptions struct {
	Addresses   []string
	Password    string
	DB          int
	PoolSize    int
	BaseTTL     time.Duration
	NegativeTTL time.Duration
}

func NewRedisCache(opts CacheOptions, log *logger.Logger, m *metrics.Metrics) *RedisCache {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:      opts.Addresses,
		Password:   opts.Password,
		DB:         opts.DB,
		PoolSize:   opts.PoolSize,
		MaxRetries: 3,

		// Connection timeouts
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,

		// Pool settings
		MinIdleConns:    10,
		ConnMaxLifetime: 5 * time.Minute,
		PoolTimeout:     4 * time.Second,
	})

	return &RedisCache{
		client:      client,
		baseTTL:     opts.BaseTTL,
		negativeTTL: opts.NegativeTTL,
		logger:      log,
		metrics:     m,
		tracer:      otel.GetTracerProvider().Tracer("redis-cache"),
	}
}

// Get retrieves value from cache
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, CacheMeta, error) {
	ctx, span := c.tracer.Start(ctx, "cache.Get",
		trace.WithAttributes(attribute.String("cache.key", key)),
	)
	defer span.End()

	start := time.Now()
	defer func() {
		if c.metrics != nil {
			c.metrics.CacheGetDuration.Observe(time.Since(start).Seconds())
		}
	}()

	val, err := c.client.Get(ctx, key).Bytes()
	if err == nil {
		if c.metrics != nil {
			c.metrics.CacheHits.Inc()
		}
		span.SetAttributes(attribute.Bool("cache.hit", true))
		return val, CacheMeta{Status: "hit"}, nil
	}

	if err == redis.Nil {
		// Check negative cache
		negKey := key + ":neg"
		_, err := c.client.Get(ctx, negKey).Result()
		if err == nil {
			if c.metrics != nil {
				c.metrics.CacheHits.Inc()
			}
			span.SetAttributes(
				attribute.Bool("cache.hit", true),
				attribute.Bool("cache.negative", true),
			)
			return nil, CacheMeta{Status: "neg"}, ErrNotFound
		}

		if c.metrics != nil {
			c.metrics.CacheMisses.Inc()
		}
		span.SetAttributes(attribute.Bool("cache.miss", true))
		return nil, CacheMeta{Status: "miss"}, ErrCacheMiss
	}

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	return nil, CacheMeta{Status: "error"}, err
}

// Set stores value in cache with optional TTL
func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl ...time.Duration) error {
	ctx, span := c.tracer.Start(ctx, "cache.Set",
		trace.WithAttributes(
			attribute.String("cache.key", key),
			attribute.Int("cache.value_size", len(value)),
		),
	)
	defer span.End()

	// Use provided TTL or default with jitter
	expiry := c.baseTTL
	if len(ttl) > 0 {
		expiry = ttl[0]
	}

	// Add jitter to TTL
	if jitter, err := rand.Int(rand.Reader, big.NewInt(300)); err == nil && len(ttl) == 0 {
		expiry = expiry + time.Duration(jitter.Int64())*time.Second
	}

	span.SetAttributes(attribute.Int64("cache.ttl_ms", expiry.Milliseconds()))

	err := c.client.Set(ctx, key, value, expiry).Err()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}

// SetNegative sets a negative cache entry
func (c *RedisCache) SetNegative(ctx context.Context, key string) error {
	ctx, span := c.tracer.Start(ctx, "cache.SetNegative",
		trace.WithAttributes(
			attribute.String("cache.key", key),
			attribute.Bool("cache.negative", true),
		),
	)
	defer span.End()

	negKey := key + ":neg"
	span.SetAttributes(
		attribute.String("cache.neg_key", negKey),
		attribute.Int64("cache.ttl_ms", c.negativeTTL.Milliseconds()),
	)

	err := c.client.Set(ctx, negKey, "1", c.negativeTTL).Err()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}

// Delete removes a key
func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	ctx, span := c.tracer.Start(ctx, "cache.Delete",
		trace.WithAttributes(
			attribute.Int("cache.key_count", len(keys)),
		),
	)
	defer span.End()

	if len(keys) == 0 {
		return nil
	}

	err := c.client.Del(ctx, keys...).Err()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return err
}

// GetOrFetch implements cache-aside pattern with single-flight
func (c *RedisCache) GetOrFetch(
	ctx context.Context,
	key string,
	fetcher func(context.Context) ([]byte, error),
) ([]byte, CacheMeta, error) {
	// Try cache first
	val, meta, err := c.Get(ctx, key)
	if err == nil {
		return val, meta, nil
	}

	if err == ErrNotFound {
		return nil, meta, ErrNotFound
	}

	// Cache miss - use single-flight
	result, err, _ := c.sf.Do(key, func() (interface{}, error) {
		// Double-check cache (another goroutine might have populated it)
		val, _, err := c.Get(ctx, key)
		if err == nil {
			return val, nil
		}

		// Fetch from source
		c.logger.Debug("Cache miss, fetching from source",
			zap.String("key", key))

		data, err := fetcher(ctx)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				// Store negative cache
				_ = c.SetNegative(ctx, key)
				return nil, ErrNotFound
			}
			return nil, err
		}

		// Store in cache
		if err := c.Set(ctx, key, data, c.baseTTL); err != nil {
			c.logger.Warn("Failed to cache value",
				zap.String("key", key),
				zap.Error(err))
		}

		return data, nil
	})

	if err != nil {
		return nil, CacheMeta{Status: "miss"}, err
	}

	return result.([]byte), CacheMeta{Status: "miss"}, nil
}

// InvalidateByPattern deletes keys matching pattern
func (c *RedisCache) InvalidateByPattern(ctx context.Context, pattern string) error {
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	pipe := c.client.Pipeline()

	count := 0
	for iter.Next(ctx) {
		pipe.Del(ctx, iter.Val())
		count++

		// Execute in batches of 1000
		if count%1000 == 0 {
			if _, err := pipe.Exec(ctx); err != nil {
				return err
			}
			pipe = c.client.Pipeline()
		}
	}

	if err := iter.Err(); err != nil {
		return err
	}

	if count%1000 != 0 {
		_, err := pipe.Exec(ctx)
		return err
	}

	c.logger.Info("Invalidated cache keys",
		zap.String("pattern", pattern),
		zap.Int("count", count))

	return nil
}

// jitterTTL adds random jitter to TTL
func (c *RedisCache) jitterTTL(ttl time.Duration, jitterFraction float64) time.Duration {
	// Generate random jitter between -jitterFraction and +jitterFraction
	maxJitter := int64(float64(ttl) * jitterFraction)

	n, err := rand.Int(rand.Reader, big.NewInt(maxJitter*2+1))
	if err != nil {
		// Fallback to no jitter on error
		return ttl
	}

	jitter := n.Int64() - maxJitter
	return ttl + time.Duration(jitter)
}

// Ping checks Redis connectivity
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}
