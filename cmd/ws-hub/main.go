package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
	"github.com/linkmeAman/universal-middleware/internal/api/middleware"
	ws "github.com/linkmeAman/universal-middleware/internal/websocket"
	"github.com/linkmeAman/universal-middleware/pkg/config"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking
		return true
	},
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New("ws-hub", "info")
	if err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	// Initialize metrics
	metrics.New("ws_hub")

	// Create enhanced WebSocket hub with debug logger
	zapLogger, _ := zap.NewDevelopment()
	hub, err := ws.NewEnhancedHub(cfg.Redis.Addresses[0], zapLogger)
	if err != nil {
		log.Fatal("Failed to create WebSocket hub", zap.Error(err))
	}

	// Initialize security middleware
	securityMw := middleware.NewSecurityMiddleware(
		"your-secret-key", // TODO: Get from config or environment
		cfg.Redis.Addresses[0],
		zapLogger,
	)

	go hub.Run()

	// Handlers are now set up in the router above

	// Start server
	wsPort := cfg.Websocket.Port
	if wsPort == 0 {
		wsPort = 8085 // Default to 8085 if not configured
	}

	router := http.NewServeMux()

	// Add WebSocket handler
	router.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		// Authenticate WebSocket connection
		userID, err := securityMw.AuthenticateWebSocket(r)
		if err != nil {
			log.Warn("WebSocket authentication failed", zap.Error(err))
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		// Pass authenticated userID to handleWebSocket
		handleWebSocket(hub, w, r, log, userID)
	})

	// Add health check handler
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":      "healthy",
			"service":     "ws-hub",
			"version":     "1.0.0",
			"connections": hub.ConnectionCount(),
		})
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Websocket.Host, wsPort),
		Handler: router,
	}

	go func() {
		log.Info("Starting WebSocket server", zap.String("addr", srv.Addr))
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
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server stopped")
}

func handleWebSocket(hub *ws.EnhancedHub, w http.ResponseWriter, r *http.Request, log *logger.Logger, userID string) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("Failed to upgrade connection", zap.Error(err))
		return
	}

	// userID is already provided from the authentication middleware
	if userID == "" {
		log.Error("Missing user ID")
		conn.Close()
		return
	}

	// Create new client
	client := ws.NewClient(hub, conn, userID)
	// Send client to hub's register channel
	select {
	case hub.Register <- client:
		log.Info("Client registered", zap.String("user_id", userID))
	default:
		log.Error("Failed to register client - hub channel full", zap.String("user_id", userID))
		conn.Close()
		return
	}

	// Start client read/write pumps
	go client.WritePump()
	go client.ReadPump()
}
