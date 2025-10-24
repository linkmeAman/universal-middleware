package testutils

import (
	"os"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// TestConfig holds global test configuration
var TestConfig = struct {
	// Timeouts
	ShortTimeout  time.Duration
	MediumTimeout time.Duration
	LongTimeout   time.Duration

	// Kafka
	KafkaBootstrapServers []string
	KafkaTopics           []string

	// Redis
	RedisAddress string
	RedisDB      int

	// Database
	DatabaseDSN string
}{
	ShortTimeout:  100 * time.Millisecond,
	MediumTimeout: 500 * time.Millisecond,
	LongTimeout:   2 * time.Second,

	KafkaBootstrapServers: []string{"localhost:9092"},
	KafkaTopics:           []string{"test-topic"},

	RedisAddress: "localhost:6379",
	RedisDB:      0,

	DatabaseDSN: "postgres://localhost:5432/test?sslmode=disable",
}

// NewTestLogger creates a logger suitable for testing
func NewTestLogger(t *testing.T) *zap.Logger {
	if testing.Short() {
		return zap.NewNop()
	}
	return zaptest.NewLogger(t)
}

// MockKafkaConfig returns Kafka config suitable for testing
func MockKafkaConfig() *sarama.Config {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	// Speed up tests by reducing timeouts
	config.Net.DialTimeout = TestConfig.ShortTimeout
	config.Net.ReadTimeout = TestConfig.ShortTimeout
	config.Net.WriteTimeout = TestConfig.ShortTimeout
	return config
}

// IsIntegrationTest checks if integration tests should run
func IsIntegrationTest() bool {
	return os.Getenv("INTEGRATION_TEST") == "true"
}

// IsBenchmarkTest checks if benchmark tests should run
func IsBenchmarkTest() bool {
	return os.Getenv("BENCHMARK_TEST") == "true"
}

// SkipIfShort skips test if -short flag is used
func SkipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping in short mode")
	}
}

// SkipIfNotIntegration skips test if not in integration mode
func SkipIfNotIntegration(t *testing.T) {
	if !IsIntegrationTest() {
		t.Skip("Skipping integration test")
	}
}

// SkipIfNotBenchmark skips test if not in benchmark mode
func SkipIfNotBenchmark(t *testing.T) {
	if !IsBenchmarkTest() {
		t.Skip("Skipping benchmark test")
	}
}
