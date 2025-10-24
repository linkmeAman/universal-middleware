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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/linkmeAman/universal-middleware/internal/api/handlers"
	"github.com/linkmeAman/universal-middleware/internal/api/middleware"
	"github.com/linkmeAman/universal-middleware/pkg/config"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New("api-gateway", "info")
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Initialize metrics
	// m := metrics.New("api_gateway")

	// Initialize metrics
	m := metrics.New("api_gateway")

	// Create router
	r := chi.NewRouter()

	// Create middleware stack
	mw := middleware.New(log, m)

	// Global middleware chain
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(mw.RequestLogger)
	r.Use(mw.RequestTracker)
	r.Use(mw.MetricsCollector)

	// Health check dependencies
	healthDeps := map[string]func() error{
		"metrics": func() error {
			// Verify metrics system
			return nil // Add actual check if needed
		},
	}

	// Health check endpoint with version and dependency checks
	r.Get("/health", handlers.HealthHandler("1.0.0", healthDeps))

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// API routes will be added here
	r.Route("/api/v1", func(r chi.Router) {
		// Protected routes will go here
	})

	// Create server
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: r,
	}

	// Start server
	go func() {
		log.Info("Starting API Gateway", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server error", zap.Error(err))
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	log.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server stopped")
}
