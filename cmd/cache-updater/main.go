package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/linkmeAman/universal-middleware/internal/api/handlers"
	"github.com/linkmeAman/universal-middleware/internal/cacheupdater"
	"github.com/linkmeAman/universal-middleware/pkg/config"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New("cache-updater", "info")
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Create service
	// Use first Redis address from the config
	redisAddr := cfg.Redis.Addresses[0]
	if redisAddr == "" {
		log.Error("No Redis address configured")
		os.Exit(1)
	}

	zapLogger, _ := zap.NewProduction()
	svc, err := cacheupdater.NewService(
		redisAddr,
		cfg.Kafka.Brokers,
		cfg.Kafka.Consumer.Topics[2], // Use the 'cache' topic for cache-updater
		zapLogger,
	)
	if err != nil {
		log.Error("Failed to create service", zap.Error(err))
		os.Exit(1)
	}

	// Create router for HTTP endpoints
	r := chi.NewRouter()

	// Basic middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	// Health check dependencies
	healthDeps := map[string]func() error{
		"redis": func() error {
			return svc.GetRedisClient().Ping(context.Background()).Err()
		},
		"kafka": func() error {
			if status := svc.GetConsumerStatus(); !status.Connected {
				return fmt.Errorf("kafka consumer not connected: %s", status.Error)
			}
			return nil
		},
	}

	// Health check endpoint
	r.Get("/health", handlers.HealthHandler("1.0.0", healthDeps))

	// Start HTTP server using cache-updater config
	port := 8084 // Default
	if cfg.CacheUpdater.Port > 0 {
		port = cfg.CacheUpdater.Port
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.CacheUpdater.Host, port),
		Handler:      r,
		ReadTimeout:  cfg.CacheUpdater.ReadTimeout,
		WriteTimeout: cfg.CacheUpdater.WriteTimeout,
	}

	// Start HTTP server in background
	go func() {
		log.Info("Starting HTTP server", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server failed", zap.Error(err))
		}
	}()

	// Start service
	if err := svc.Start(context.Background()); err != nil {
		log.Error("Failed to start service", zap.Error(err))
		os.Exit(1)
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Info("Shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// First stop the HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown failed", zap.Error(err))
	}

	// Then stop the service
	if err := svc.Stop(); err != nil {
		log.Error("Error during service shutdown", zap.Error(err))
		os.Exit(1)
	}

	log.Info("Shutdown complete")
}
