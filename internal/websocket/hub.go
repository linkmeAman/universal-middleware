package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RealtimeMessage represents a message sent to clients
type RealtimeMessage struct {
	Type      string          `json:"type"`
	Entity    string          `json:"entity"`
	Action    string          `json:"action"`
	Data      json.RawMessage `json:"data"`
	Timestamp int64           `json:"timestamp"`
}

// EnhancedHub manages WebSocket connections with Redis integration
type EnhancedHub struct {
	// Connection management
	clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	broadcast  chan *Broadcast

	// Redis integration for distributed pub/sub
	redisClient *redis.Client
	redisSub    *redis.PubSub

	// Room management for targeted updates
	rooms   map[string]map[*Client]bool
	roomsMu sync.RWMutex

	// Metrics and logging
	log       *zap.Logger
	connCount int64
	msgCount  int64

	// Graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// NewEnhancedHub creates a hub with Redis pub/sub support
func NewEnhancedHub(redisAddr string, log *zap.Logger) (*EnhancedHub, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Password:     "",
		DB:           0,
		PoolSize:     50,
		MinIdleConns: 10,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	// Test connection
	if err := rdb.Ping(ctx).Err(); err != nil {
		cancel()
		return nil, err
	}

	hub := &EnhancedHub{
		clients:     make(map[*Client]bool),
		Register:    make(chan *Client, 256),
		Unregister:  make(chan *Client, 256),
		broadcast:   make(chan *Broadcast, 1024),
		rooms:       make(map[string]map[*Client]bool),
		redisClient: rdb,
		log:         log,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Subscribe to Redis pub/sub channels
	hub.redisSub = rdb.Subscribe(ctx, "realtime:*")

	return hub, nil
}

// Run starts the hub's main event loop
func (h *EnhancedHub) Run() {
	// Start Redis pub/sub listener
	go h.listenRedis()

	for {
		select {
		case <-h.ctx.Done():
			h.shutdown()
			return

		case client := <-h.Register:
			h.registerClient(client)

		case client := <-h.Unregister:
			h.unregisterClient(client)

		case broadcast := <-h.broadcast:
			h.broadcastMessage(broadcast)
		}
	}
}

// listenRedis handles messages from Redis pub/sub
func (h *EnhancedHub) listenRedis() {
	ch := h.redisSub.Channel()

	for {
		select {
		case <-h.ctx.Done():
			return

		case msg := <-ch:
			if msg == nil {
				continue
			}

			// Parse the message
			var realtimeMsg RealtimeMessage
			if err := json.Unmarshal([]byte(msg.Payload), &realtimeMsg); err != nil {
				h.log.Error("Failed to parse Redis message", zap.Error(err))
				continue
			}

			// Extract room from channel (e.g., "realtime:entity.123")
			room := msg.Channel[len("realtime:"):]

			// Broadcast to relevant clients
			h.broadcast <- &Broadcast{
				Room:    room,
				Message: []byte(msg.Payload),
			}
		}
	}
}

// PublishUpdate publishes an update via Redis (called by Cache Updater)
func (h *EnhancedHub) PublishUpdate(ctx context.Context, room string, msg RealtimeMessage) error {
	msg.Timestamp = time.Now().Unix()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Publish to Redis channel
	channel := "realtime:" + room
	return h.redisClient.Publish(ctx, channel, data).Err()
}

// registerClient adds a new client to the hub
func (h *EnhancedHub) registerClient(client *Client) {
	h.clients[client] = true
	h.connCount++

	h.log.Info("Client connected",
		zap.String("user_id", client.UserID),
		zap.Int64("total_connections", h.connCount))
}

// unregisterClient removes a client from the hub
func (h *EnhancedHub) unregisterClient(client *Client) {
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		h.connCount--

		// Remove from all rooms
		h.roomsMu.Lock()
		for room := range client.rooms {
			if clients, ok := h.rooms[room]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.rooms, room)
				}
			}
		}
		h.roomsMu.Unlock()

		close(client.send)

		h.log.Info("Client disconnected",
			zap.String("user_id", client.UserID),
			zap.Int64("total_connections", h.connCount))
	}
}

// broadcastMessage sends a message to all clients in a room
func (h *EnhancedHub) broadcastMessage(broadcast *Broadcast) {
	h.roomsMu.RLock()
	clients, exists := h.rooms[broadcast.Room]
	h.roomsMu.RUnlock()

	if !exists {
		return
	}

	h.msgCount++
	successCount := 0
	droppedCount := 0

	for client := range clients {
		select {
		case client.send <- broadcast.Message:
			successCount++
		default:
			// Client's buffer is full - drop message or disconnect
			droppedCount++
			h.log.Warn("Message dropped - client buffer full",
				zap.String("user_id", client.UserID),
				zap.String("room", broadcast.Room))
		}
	}

	if droppedCount > 0 {
		h.log.Warn("Broadcast completed with drops",
			zap.String("room", broadcast.Room),
			zap.Int("delivered", successCount),
			zap.Int("dropped", droppedCount))
	}
}

// JoinRoom adds a client to a room for targeted updates
func (h *EnhancedHub) JoinRoom(client *Client, room string) {
	h.roomsMu.Lock()
	defer h.roomsMu.Unlock()

	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*Client]bool)
	}

	h.rooms[room][client] = true
	client.rooms[room] = true

	h.log.Debug("Client joined room",
		zap.String("user_id", client.UserID),
		zap.String("room", room))
}

// LeaveRoom removes a client from a room
func (h *EnhancedHub) LeaveRoom(client *Client, room string) {
	h.roomsMu.Lock()
	defer h.roomsMu.Unlock()

	if clients, ok := h.rooms[room]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.rooms, room)
		}
	}

	delete(client.rooms, room)

	h.log.Debug("Client left room",
		zap.String("user_id", client.UserID),
		zap.String("room", room))
}

// ConnectionCount returns the current number of connected clients
func (h *EnhancedHub) ConnectionCount() int {
	return int(h.connCount)
}

// shutdown gracefully shuts down the hub
func (h *EnhancedHub) shutdown() {
	h.log.Info("Shutting down WebSocket hub")

	// Close all client connections
	for client := range h.clients {
		close(client.send)
	}

	// Close Redis subscription
	if h.redisSub != nil {
		h.redisSub.Close()
	}

	// Close Redis client
	if h.redisClient != nil {
		h.redisClient.Close()
	}
}

// Broadcast represents a message to broadcast to a room
type Broadcast struct {
	Room    string
	Message []byte
}
