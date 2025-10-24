package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/linkmeAman/universal-middleware/internal/processor"
	"github.com/linkmeAman/universal-middleware/pkg/config"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New("processor", "info")
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Create processor service with config values
	proc, err := processor.NewService(
		cfg.Kafka.Brokers,
		cfg.Kafka.Consumer.Topics[0], // Use the 'commands' topic for processor
		cfg.Kafka.GroupID,
		cfg.Kafka.Consumer.MinBytes,
		cfg.Kafka.Consumer.MaxBytes,
		log,
	)
	if err != nil {
		log.Error("Failed to create processor service", zap.Error(err))
		os.Exit(1)
	}

	// Start processing
	// Note: Context will be used for graceful shutdown in the future
	if err := proc.Start(); err != nil {
		log.Error("Failed to start processor service", zap.Error(err))
		os.Exit(1)
	}

	// Initialize metrics
	metrics.New("event_processor")

	// Create HTTP server for health checks
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "healthy",
			"version": "1.0.0",
			"services": map[string]string{
				"kafka": "healthy",
			},
		})
	})

	// Start HTTP server using processor config
	port := 8083 // Default
	if cfg.Processor.Port > 0 {
		port = cfg.Processor.Port
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Processor.Host, port),
		Handler:      http.DefaultServeMux,
		ReadTimeout:  cfg.Processor.ReadTimeout,
		WriteTimeout: cfg.Processor.WriteTimeout,
	}

	go func() {
		log.Info("Starting HTTP server", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info("Shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Gracefully shutdown HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown failed", zap.Error(err))
	}

	// Stop processor
	if err := proc.Stop(); err != nil {
		log.Error("Failed to stop processor", zap.Error(err))
	}
	log.Info("Shutdown complete")
}
