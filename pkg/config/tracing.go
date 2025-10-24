package config

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/tracing"
)

// SetupTracing initializes the OpenTelemetry tracing
func SetupTracing(serviceName string, log *logger.Logger) (*tracing.Tracer, error) {
	cfg := tracing.Config{
		ServiceName:    serviceName,
		ServiceVersion: os.Getenv("SERVICE_VERSION"),
		Environment:    os.Getenv("ENVIRONMENT"),
		Endpoint:       os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}

	if cfg.Endpoint == "" {
		cfg.Endpoint = "localhost:4317" // Default to local collector
	}

	tracer, err := tracing.New(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to setup tracing: %w", err)
	}

	// Handle graceful shutdown
	ctx := context.Background()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		if err := tracer.Shutdown(ctx); err != nil {
			log.Error("Failed to shutdown tracer", zap.Error(err))
		}
	}()

	return tracer, nil
}
