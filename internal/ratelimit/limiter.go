package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RateLimiter handles rate limiting using Redis
type RateLimiter struct {
	client     *redis.Client
	logger     *zap.Logger
	window     time.Duration
	maxTokens  int
	windowSize int64
}

// Config holds rate limiter configuration
type Config struct {
	MaxTokens   int           // Maximum number of tokens per window
	Window      time.Duration // Time window for rate limiting
	BurstSize   int           // Maximum burst size allowed
	RedisConfig *redis.Options
}

// New creates a new rate limiter instance
func New(cfg Config, logger *zap.Logger) (*RateLimiter, error) {
	client := redis.NewClient(cfg.RedisConfig)

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	return &RateLimiter{
		client:     client,
		logger:     logger,
		window:     cfg.Window,
		maxTokens:  cfg.MaxTokens,
		windowSize: int64(cfg.Window.Seconds()),
	}, nil
}

// Allow checks if a request should be allowed based on the rate limit
func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := time.Now().Unix()
	window := now - (now % rl.windowSize)

	// Lua script for atomic rate limiting
	script := `
		local key = KEYS[1]
		local window = tonumber(ARGV[1])
		local max_tokens = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])

		-- Clean up old windows
		redis.call('ZREMRANGEBYSCORE', key, 0, window - 1)

		-- Get current count
		local count = redis.call('ZCOUNT', key, window, window + 86400)

		-- Check if we can add more
		if count >= max_tokens then
			return 0
		end

		-- Add request timestamp
		redis.call('ZADD', key, now, now .. '-' .. math.random())
		-- Set expiry
		redis.call('EXPIRE', key, 86400)
		
		return 1
	`

	result, err := rl.client.Eval(ctx, script, []string{key}, window, rl.maxTokens, now).Result()
	if err != nil {
		return false, fmt.Errorf("failed to evaluate rate limit: %v", err)
	}

	allowed := result.(int64) == 1
	if !allowed {
		rl.logger.Debug("Rate limit exceeded",
			zap.String("key", key),
			zap.Int("max_tokens", rl.maxTokens),
			zap.Duration("window", rl.window))
	}

	return allowed, nil
}

// GetRemainingTokens returns the number of remaining tokens for a key
func (rl *RateLimiter) GetRemainingTokens(ctx context.Context, key string) (int, error) {
	now := time.Now().Unix()
	window := now - (now % rl.windowSize)

	count, err := rl.client.ZCount(ctx, key, fmt.Sprint(window), fmt.Sprint(window+86400)).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get token count: %v", err)
	}

	remaining := rl.maxTokens - int(count)
	if remaining < 0 {
		remaining = 0
	}

	return remaining, nil
}

// MaxTokens returns the maximum number of tokens per window
func (rl *RateLimiter) MaxTokens() int {
	return rl.maxTokens
}

// NextReset returns the Unix timestamp for the next rate limit reset
func (rl *RateLimiter) NextReset() int64 {
	now := time.Now().Unix()
	window := now - (now % rl.windowSize)
	return window + rl.windowSize
}

// Close closes the Redis connection
func (rl *RateLimiter) Close() error {
	return rl.client.Close()
}
