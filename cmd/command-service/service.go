package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	"github.com/linkmeAman/universal-middleware/internal/database/postgres"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/linkmeAman/universal-middleware/internal/api/handlers"
	"github.com/linkmeAman/universal-middleware/internal/command"
	"github.com/linkmeAman/universal-middleware/internal/command/outbox"
	"github.com/linkmeAman/universal-middleware/internal/events/publisher"
	"github.com/linkmeAman/universal-middleware/pkg/config"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
	"go.uber.org/zap"
)

// Run starts the command service
func initTracer(cfg *config.Config, metrics *metrics.Metrics) (*trace.TracerProvider, error) {
	// Create resource for tracer
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("command-service"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize tracer provider
	tp := trace.NewTracerProvider(
		trace.WithResource(r),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

// Global variables for server components
var (
	srv   *http.Server
	errCh chan error
)

func run() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize logger
	log, err := logger.New("command-service", "info")
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer log.Sync()

	// Initialize metrics
	metrics := metrics.New("command_service")

	// Skip tracing initialization if disabled
	if cfg.Observability.Tracing.Disable {
		log.Info("Tracing is disabled, skipping initialization")
	} else {
		// Initialize tracer with metrics
		tracer, terr := initTracer(cfg, metrics)
		if terr != nil {
			return fmt.Errorf("failed to initialize tracer: %w", terr)
		}
		defer tracer.Shutdown(context.Background())
	}

	// Create service context
	serviceCtx, serviceCancel := context.WithCancel(context.Background())
	defer serviceCancel()

	// Create HTTP router first
	r := chi.NewRouter()

	// Basic middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// Initial health check endpoint without dependencies
	r.Get("/health", handlers.HealthHandler("1.0.0", make(map[string]func() error)))

	// Start HTTP server before other initialization
	var serverStartErr error
	errCh = make(chan error, 1)

	srv = &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%d", 8082),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server in background
	go func() {
		log.Info("Starting HTTP server", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server failed to start", zap.Error(err))
			errCh <- err
		}
	}()

	// Verify server is listening with retries
	serverMaxRetries := 5
	for i := 0; i < serverMaxRetries; i++ {
		var resp *http.Response
		resp, serverStartErr = http.Get(fmt.Sprintf("http://localhost:%d/health", 8082))
		if serverStartErr == nil {
			resp.Body.Close()
			log.Info("HTTP server started and verified")
			break
		}
		if i < serverMaxRetries-1 {
			log.Warn("Server not ready, retrying...",
				zap.Int("attempt", i+1),
				zap.Error(serverStartErr))
			time.Sleep(time.Second)
		}
	}
	if serverStartErr != nil {
		log.Error("Failed to verify server is listening", zap.Error(serverStartErr))
		return fmt.Errorf("server verification failed: %w", serverStartErr)
	}

	// Initialize other components
	// Create event publisher
	pub, err := publisher.NewProducer(publisher.ProducerConfig{
		Brokers:           cfg.Kafka.Brokers,
		RequiredAcks:      sarama.WaitForAll,
		Compression:       sarama.CompressionSnappy,
		MaxRetries:        3,
		RetryBackoff:      100 * time.Millisecond,
		ConnectionTimeout: 10 * time.Second,
	}, log)
	if err != nil {
		return fmt.Errorf("failed to create event publisher: %w", err)
	}

	// Create outbox repository with retries
	log.Info("Connecting to database",
		zap.String("host", cfg.Database.Primary.Host),
		zap.Int("port", cfg.Database.Primary.Port),
		zap.String("db", cfg.Database.Primary.Database),
		zap.String("user", cfg.Database.Primary.Username))

	var db *postgres.DB
	maxRetries := 5
	retryInterval := time.Second * 2

	for i := 0; i < maxRetries; i++ {
		db, err = postgres.InitFromConfig(cfg, log, metrics)
		if err == nil {
			break
		}

		if i < maxRetries-1 {
			log.Warn("Failed to connect to database, retrying...",
				zap.String("host", cfg.Database.Primary.Host),
				zap.Int("attempt", i+1),
				zap.Error(err))
			time.Sleep(retryInterval)
			continue
		}

		log.Error("Failed to connect to database after retries",
			zap.String("host", cfg.Database.Primary.Host),
			zap.Int("port", cfg.Database.Primary.Port),
			zap.String("db", cfg.Database.Primary.Database),
			zap.String("user", cfg.Database.Primary.Username),
			zap.Error(err))
		return fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
	}
	defer db.Close()

	outboxRepo := outbox.NewRepository(db, log)

	// Create outbox processor with default values
	outboxProcessorConfig := outbox.DefaultConfig()
	// Override with config values if provided
	if cfg.Outbox.BatchSize > 0 {
		outboxProcessorConfig.BatchSize = cfg.Outbox.BatchSize
	}
	if cfg.Outbox.PollingInterval > 0 {
		outboxProcessorConfig.PollingInterval = cfg.Outbox.PollingInterval
	}
	if cfg.Outbox.RetryInterval > 0 {
		outboxProcessorConfig.RetryDelay = cfg.Outbox.RetryInterval
	}
	if cfg.Outbox.MaxRetries > 0 {
		outboxProcessorConfig.MaxRetries = cfg.Outbox.MaxRetries
	}
	outboxProcessor := outbox.NewProcessor(outboxProcessorConfig, outboxRepo, pub, log)

	// Create command processor
	cmdProcessor := command.NewProcessor(command.ProcessorConfig{
		MaxWorkers:     cfg.Command.MaxWorkers,
		QueueSize:      cfg.Command.QueueSize,
		DefaultTimeout: cfg.Command.DefaultTimeout,
	}, log)

	// Start outbox processor
	if err := outboxProcessor.Start(serviceCtx); err != nil {
		log.Error("Failed to start outbox processor", zap.Error(err))
		return err
	}

	// Health check dependencies
	healthDeps := map[string]func() error{
		"database": func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return db.Ping(ctx)
		},
		"kafka": func() error {
			if err := pub.Ping(); err != nil {
				log.Error("Kafka health check failed", zap.Error(err))
				return fmt.Errorf("kafka check failed: %w", err)
			}
			return nil
		},
		"processor": func() error {
			if err := cmdProcessor.Status(); err != nil {
				log.Error("Command processor health check failed", zap.Error(err))
				return fmt.Errorf("command processor check failed: %w", err)
			}
			return nil
		},
		"outbox": func() error {
			if outboxProcessor == nil {
				return fmt.Errorf("outbox processor not initialized")
			}
			return nil
		},
	}

	// Health check endpoint with version and dependency checks
	r.Get("/health", handlers.HealthHandler("1.0.0", healthDeps))

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// Command endpoints
	r.Post("/v1/commands", HandleCommand(cmdProcessor, log))

	// Start server
	srv := &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%d", 8082), // Fixed port
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server in background with error check channel
	errCh := make(chan error, 1)
	listenAddr := fmt.Sprintf("0.0.0.0:%d", 8082)

	// Log just before starting server
	log.Info("Starting HTTP server", zap.String("addr", listenAddr))

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server failed to start", zap.Error(err))
			errCh <- err
			return
		}
	}()

	// Try to make a test request to verify server is up
	startupTimeout := time.NewTimer(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	defer startupTimeout.Stop()

checkLoop:
	for {
		select {
		case err := <-errCh:
			return fmt.Errorf("server startup failed: %w", err)
		case <-startupTimeout.C:
			return fmt.Errorf("server startup timed out after 5 seconds")
		case <-ticker.C:
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", 8082))
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					log.Info("HTTP server started successfully and is accepting connections")
					break checkLoop
				}
			}
		}
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigCh:
		log.Info("Shutting down...")
	case err := <-errCh:
		log.Error("Server error", zap.Error(err))
		return err
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server shutdown failed", zap.Error(err))
	}

	// Stop processors
	serviceCancel()
	return nil
}
