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
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/sessions"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/linkmeAman/universal-middleware/internal/api/grpc"
	"github.com/linkmeAman/universal-middleware/internal/loadbalancer"
	"github.com/linkmeAman/universal-middleware/internal/ratelimit"

	"github.com/linkmeAman/universal-middleware/internal/api/handlers"
	"github.com/linkmeAman/universal-middleware/internal/api/middleware"
	"github.com/linkmeAman/universal-middleware/internal/api/validation"
	"github.com/linkmeAman/universal-middleware/internal/auth"
	"github.com/linkmeAman/universal-middleware/pkg/config"
	"github.com/linkmeAman/universal-middleware/pkg/env"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
)

func main() {
	// Create a root context that can be used to gracefully shutdown goroutines
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Load environment configuration
	envCfg, err := env.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load environment config: %v\n", err)
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

	// Initialize load balancer
	zapLogger, _ := zap.NewProduction()
	lb := loadbalancer.New(zapLogger)

	// Add backend servers from config
	for _, backend := range cfg.Backends {
		lb.AddBackend(backend.URL)
	}

	// Initialize rate limiter if enabled
	var rateLimiter *ratelimit.RateLimiter
	if cfg.RateLimit.Enabled {
		limiterConfig := ratelimit.Config{
			MaxTokens: cfg.RateLimit.MaxTokens,
			Window:    cfg.RateLimit.Window,
			BurstSize: cfg.RateLimit.BurstSize,
			RedisConfig: &redis.Options{
				Addr:     cfg.Redis.Addresses[0],
				Password: cfg.Redis.Password,
				DB:       cfg.Redis.DB,
			},
		}

		var err error
		rateLimiter, err = ratelimit.New(limiterConfig, zapLogger)
		if err != nil {
			log.Error("Failed to initialize rate limiter", zap.Error(err))
			os.Exit(1)
		}
		defer rateLimiter.Close()
	}

	// Global middleware chain
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(mw.RequestLogger)
	r.Use(mw.RequestTracker)
	r.Use(mw.MetricsCollector)
	r.Use(mw.LoadBalancer(lb))

	if rateLimiter != nil {
		r.Use(mw.RateLimit(rateLimiter))
	}

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

	// Initialize session store
	sessionStore := sessions.NewCookieStore([]byte(envCfg.SessionSecret))
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		Domain:   envCfg.Domain,
		MaxAge:   3600, // 1 hour
		Secure:   cfg.Auth.Session.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}

	// Initialize OAuth2 provider
	var oauth2Provider auth.OAuth2Provider
	if os.Getenv("APP_ENV") == "production" {
		oauth2Config := auth.OAuth2Config{
			ProviderURL:  envCfg.OAuth2ProviderURL,
			ClientID:     envCfg.OAuth2ClientID,
			ClientSecret: envCfg.OAuth2ClientSecret,
			RedirectURL:  envCfg.OAuth2RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
		}

		var err error
		oauth2Provider, err = auth.NewOAuth2Provider(oauth2Config)
		if err != nil {
			log.Error("Failed to initialize OAuth2 provider", zap.Error(err))
			os.Exit(1)
		}
	} else {
		oauth2Provider = auth.NewDevOAuth2Provider()
		log.Info("Using development OAuth2 provider")
	}

	// Initialize OPA authorizer
	var opaAuthorizer auth.OPAAuthorizer
	if os.Getenv("APP_ENV") == "production" {
		var err error
		opaAuthorizer, err = auth.NewOPAAuthorizer(cfg.Auth.OPAEndpoint, cfg.Auth.OPAPolicy, log)
		if err != nil {
			log.Error("Failed to initialize OPA authorizer", zap.Error(err))
			os.Exit(1)
		}
	} else {
		opaAuthorizer = auth.NewDevOPAAuthorizer(log)
		log.Info("Using development OPA authorizer")
	}

	// Initialize auth middleware with session store and OAuth2 provider
	authMiddleware := middleware.NewAuthMiddleware(log, opaAuthorizer, oauth2Provider, sessionStore)

	// Initialize validator
	validator := validation.NewValidator(log)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(log, m, oauth2Provider, sessionStore)
	userHandler := handlers.NewUserHandler(log, m)

	// Start a goroutine to cleanup expired sessions
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// TODO: Implement session cleanup
			case <-ctx.Done():
				return
			}
		}
	}()

	// Start a goroutine to refresh OPA policies periodically
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := opaAuthorizer.RefreshPolicies(ctx); err != nil {
					log.Error("Failed to refresh OPA policies", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public auth routes
		r.Group(func(r chi.Router) {
			r.Get("/auth/login", authHandler.Login)
			r.Get("/auth/callback", authHandler.Callback)
			r.Get("/auth/logout", authHandler.Logout)
		})

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Authorize)

			// User routes
			r.Get("/userinfo", authHandler.UserInfo)
			r.Post("/refresh", authHandler.RefreshToken)

			// User management routes
			r.Route("/users", func(r chi.Router) {
				r.With(validator.ValidateRequest).Post("/", userHandler.CreateUser)
				// Add other user management routes here
			})
		})
	})

	// Create HTTP server
	httpSrv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: r,
	}

	// Create gRPC server
	grpcPort := cfg.Server.Port + 1 // Use next port for gRPC
	grpcServer := grpc.NewServer(zapLogger, m, grpcPort)

	// Start HTTP server
	go func() {
		log.Info("Starting HTTP server", zap.String("addr", httpSrv.Addr))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", zap.Error(err))
			os.Exit(1)
		}
	}()

	// Start gRPC server
	if err := grpcServer.Start(); err != nil {
		log.Error("Failed to start gRPC server", zap.Error(err))
		os.Exit(1)
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// Graceful shutdown
	log.Info("Shutting down servers...")
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	// Stop gRPC server
	grpcServer.Stop()

	// Stop HTTP server
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server forced to shutdown", zap.Error(err))
	}

	// Cancel background goroutines
	cancel()

	log.Info("Servers stopped")
}
