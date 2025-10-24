package cache

import (
	"context"
	"testing"
	"time"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheWarmer(t *testing.T) {
	// Initialize logger
	log, err := logger.New("test", "debug")
	require.NoError(t, err)

	// Initialize Redis cache with test configuration
	redisCache := NewRedisCache(CacheOptions{
		Addresses:   []string{"localhost:6379"},
		BaseTTL:     time.Minute,
		NegativeTTL: 30 * time.Second,
	}, log, nil)

	// Initialize cache warmer
	warmer := NewCacheWarmer(redisCache, log)

	t.Run("Basic Warm-up", func(t *testing.T) {
		// Test data for basic warm-up
		testData := map[string]interface{}{
			"test:key1": "value1",
			"test:key2": "value2",
		}

		// Register pattern for basic data
		warmer.RegisterPattern(KeyPattern{
			Pattern: "test:*",
			TTL:     time.Minute,
			Loader: func(ctx context.Context) (map[string]interface{}, error) {
				return testData, nil
			},
		})

		// Test warm-up
		err = warmer.WarmupAll(context.Background())
		require.NoError(t, err)

		// Verify data was cached
		for key, expectedValue := range testData {
			value, meta, err := redisCache.Get(context.Background(), key)
			require.NoError(t, err)
			assert.Equal(t, "hit", meta.Status)

			// Compare string values
			strValue := string(value)
			strExpected := expectedValue.(string)
			assert.Equal(t, strExpected, strValue)
		}

		// Clean up
		err = redisCache.InvalidateByPattern(context.Background(), "test:*")
		require.NoError(t, err)
	})

	t.Run("Multiple Patterns", func(t *testing.T) {
		// Test data for multiple patterns
		pattern1Data := map[string]interface{}{
			"users:1": "john",
			"users:2": "jane",
		}
		pattern2Data := map[string]interface{}{
			"orders:1": "order1",
			"orders:2": "order2",
		}

		// Register multiple patterns
		warmer.RegisterPattern(KeyPattern{
			Pattern: "users:*",
			TTL:     time.Minute,
			Loader: func(ctx context.Context) (map[string]interface{}, error) {
				return pattern1Data, nil
			},
		})

		warmer.RegisterPattern(KeyPattern{
			Pattern: "orders:*",
			TTL:     time.Minute,
			Loader: func(ctx context.Context) (map[string]interface{}, error) {
				return pattern2Data, nil
			},
		})

		// Test warm-up
		err = warmer.WarmupAll(context.Background())
		require.NoError(t, err)

		// Verify both patterns were cached
		allData := map[string]interface{}{}
		for k, v := range pattern1Data {
			allData[k] = v
		}
		for k, v := range pattern2Data {
			allData[k] = v
		}

		for key, expectedValue := range allData {
			value, meta, err := redisCache.Get(context.Background(), key)
			require.NoError(t, err)
			assert.Equal(t, "hit", meta.Status)
			assert.Equal(t, expectedValue, string(value))
		}

		// Clean up
		err = redisCache.InvalidateByPattern(context.Background(), "users:*")
		require.NoError(t, err)
		err = redisCache.InvalidateByPattern(context.Background(), "orders:*")
		require.NoError(t, err)
	})
}
