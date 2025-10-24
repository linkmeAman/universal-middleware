package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server         ServerConfig
	Websocket      ServerConfig
	CommandService ServerConfig `mapstructure:"commandservice"`
	Processor      ServerConfig
	CacheUpdater   ServerConfig
	Redis          RedisConfig
	Kafka          KafkaConfig
	Database       DatabaseConfig
	Auth           AuthConfig
	Observability  ObservabilityConfig
	Command        CommandConfig
	Outbox         OutboxConfig
}

type CommandConfig struct {
	MaxWorkers     int           `mapstructure:"max_workers"`
	QueueSize      int           `mapstructure:"queue_size"`
	DefaultTimeout time.Duration `mapstructure:"default_timeout"`
}

type OutboxConfig struct {
	BatchSize       int           `mapstructure:"batch_size"`
	PollingInterval time.Duration `mapstructure:"polling_interval"`
	RetryInterval   time.Duration `mapstructure:"retry_interval"`
	MaxRetries      int           `mapstructure:"max_retries"`
}

type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	TLS          TLSConfig     `mapstructure:"tls"`
}

type RedisConfig struct {
	Addresses       []string      `mapstructure:"addresses"`
	Password        string        `mapstructure:"password"`
	DB              int           `mapstructure:"db"`
	PoolSize        int           `mapstructure:"pool_size"`
	MinIdleConns    int           `mapstructure:"min_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type KafkaConfig struct {
	Enabled     bool           `mapstructure:"enabled"`
	Brokers     []string       `mapstructure:"brokers"`
	GroupID     string         `mapstructure:"group_id"`
	Version     string         `mapstructure:"version"`
	SASLEnabled bool           `mapstructure:"sasl_enabled"`
	Consumer    ConsumerConfig `mapstructure:"consumer"`
	Producer    ProducerConfig `mapstructure:"producer"`
}

type ConsumerConfig struct {
	MinBytes     int           `mapstructure:"min_bytes"`
	MaxBytes     int           `mapstructure:"max_bytes"`
	MaxWait      time.Duration `mapstructure:"max_wait"`
	FetchMin     int           `mapstructure:"fetch_min"`
	FetchDefault int           `mapstructure:"fetch_default"`
	RetryBackoff time.Duration `mapstructure:"retry_backoff"`
	MaxRetries   int           `mapstructure:"max_retries"`
	Topics       []string      `mapstructure:"topics"`
}

type ProducerConfig struct {
	Compression     string        `mapstructure:"compression"`
	MaxMessageBytes int           `mapstructure:"max_message_bytes"`
	RetryBackoff    time.Duration `mapstructure:"retry_backoff"`
	MaxRetries      int           `mapstructure:"max_retries"`
}

type DatabaseConfig struct {
	Primary ConnectionConfig `mapstructure:"primary"`
	Replica ConnectionConfig `mapstructure:"replica"`
	URL     string           `mapstructure:"url"`
}

type ConnectionConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Database        string        `mapstructure:"database"`
	Username        string        `mapstructure:"username"`
	Password        string        `mapstructure:"password"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
}

type AuthConfig struct {
	JWTIssuer   string `mapstructure:"jwt_issuer"`
	JWTAudience string `mapstructure:"jwt_audience"`
	OPAEndpoint string `mapstructure:"opa_endpoint"`
	OPAPolicy   string `mapstructure:"opa_policy"`
}

type ObservabilityConfig struct {
	Enabled     bool          `mapstructure:"enabled"`
	MetricsPort int           `mapstructure:"metrics_port"`
	MetricsPath string        `mapstructure:"metrics_path"`
	Tracing     TracingConfig `mapstructure:"tracing"`
}

type TracingConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	Endpoint    string `mapstructure:"endpoint"`
	ServiceName string `mapstructure:"service_name"`
	SchemaURL   string `mapstructure:"schema_url"`
	Disable     bool   `mapstructure:"disable"`
}

type TLSConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/universal-middleware/")

	// Allow environment variable overrides
	viper.AutomaticEnv()
	viper.SetEnvPrefix("UMW")

	// Set defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080) // API Gateway
	viper.SetDefault("websocket.host", "0.0.0.0")
	viper.SetDefault("websocket.port", 8081)     // WebSocket Hub
	viper.SetDefault("command.port", 8082)       // Command Service
	viper.SetDefault("processor.port", 8083)     // Event Processor
	viper.SetDefault("cache_updater.port", 8084) // Cache Updater
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("redis.pool_size", 100)
	viper.SetDefault("database.primary.max_open_conns", 50)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
