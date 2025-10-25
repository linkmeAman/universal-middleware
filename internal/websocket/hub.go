package websocket

import (
	"encoding/json"
	"sync"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
	"go.uber.org/zap"
)

const (
	MessageTypeJoinRoom  = "join_room"
	MessageTypeLeaveRoom = "leave_room"
	MessageTypePublish   = "publish"
)

// Message represents a WebSocket message
type Message struct {
	Type    string          `json:"type"`
	Room    string          `json:"room,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Broadcast represents a message to be broadcast
type Broadcast struct {
	Room    string
	Message []byte
	Sender  *Client
}

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Rooms and their clients
	rooms map[string]map[*Client]bool

	// Inbound messages from the clients
	broadcast chan *Broadcast

	// Register requests from the clients
	Register chan *Client

	// Unregister requests from clients
	Unregister chan *Client

	// Mutex for rooms map
	mu sync.RWMutex

	// Logger
	log *logger.Logger

	// Metrics
	metrics *metrics.Metrics
}

// NewHub creates a new Hub
func NewHub(log *logger.Logger, m *metrics.Metrics) *Hub {
	return &Hub{
		broadcast:  make(chan *Broadcast),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
		log:        log,
		metrics:    m,
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.clients[client] = true
			h.metrics.WSConnections.Inc()
			h.log.Info("Client connected", zap.String("user_id", client.userID))

		case client := <-h.Unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				h.metrics.WSConnections.Dec()
				h.log.Info("Client disconnected", zap.String("user_id", client.userID))

				// Remove from all rooms
				h.mu.Lock()
				for room := range client.rooms {
					if clients, ok := h.rooms[room]; ok {
						delete(clients, client)
						if len(clients) == 0 {
							delete(h.rooms, room)
						}
					}
				}
				h.mu.Unlock()

				close(client.send)
			}

		case broadcast := <-h.broadcast:
			h.mu.RLock()
			clients := h.rooms[broadcast.Room]
			h.mu.RUnlock()

			for client := range clients {
				if client != broadcast.Sender { // Don't send back to sender
					select {
					case client.send <- broadcast.Message:
					default:
						h.metrics.WSMessageDropped.Inc()
						h.log.Warn("Message dropped - client send buffer full",
							zap.String("user_id", client.userID),
							zap.String("room", broadcast.Room),
						)
					}
				}
			}
		}
	}
}

// joinRoom adds a client to a room
func (h *Hub) joinRoom(room string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*Client]bool)
	}
	h.rooms[room][client] = true
}

// leaveRoom removes a client from a room
func (h *Hub) leaveRoom(room string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.rooms[room]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.rooms, room)
		}
	}
}

// GetRoomCount returns the number of clients in a room
func (h *Hub) GetRoomCount(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.rooms[room]; ok {
		return len(clients)
	}
	return 0
}

// ConnectionCount returns the total number of connected clients
func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.clients)
}
