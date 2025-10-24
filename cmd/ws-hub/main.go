package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
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
	m := metrics.New("ws_hub")

	// Create WebSocket hub
	hub := ws.NewHub(log, m)
	go hub.Run()

	// WebSocket handler
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(hub, w, r, log)
	})

	// Health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	})

	// Start server
	wsPort := cfg.Websocket.Port
	if wsPort == 0 {
		wsPort = 8081 // Default to 8081 if not configured
	}
	srv := &http.Server{
		Addr: fmt.Sprintf("%s:%d", cfg.Websocket.Host, wsPort),
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

func handleWebSocket(hub *ws.Hub, w http.ResponseWriter, r *http.Request, log *logger.Logger) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error("Failed to upgrade connection", zap.Error(err))
		return
	}

	// Get user ID from request (you should implement proper authentication)
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
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
