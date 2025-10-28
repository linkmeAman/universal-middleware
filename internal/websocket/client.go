package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512KB
)

// Message types for WebSocket communication
const (
	MessageTypeJoinRoom  = "join_room"
	MessageTypeLeaveRoom = "leave_room"
	MessageTypePublish   = "publish"
)

// Message represents a WebSocket message
type Message struct {
	Type    string          `json:"type"`
	Room    string          `json:"room"`
	Payload json.RawMessage `json:"payload"`
}

// Client represents a single WebSocket connection
type Client struct {
	hub    *EnhancedHub
	conn   *websocket.Conn
	send   chan []byte
	rooms  map[string]bool
	UserID string
	mu     sync.RWMutex
}

// NewClient creates a new WebSocket client
func NewClient(hub *EnhancedHub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		rooms:  make(map[string]bool),
		UserID: userID,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.log.Error("Unexpected close error", zap.Error(err))
			}
			break
		}

		// Parse message for routing
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			c.hub.log.Error("Failed to parse message", zap.Error(err))
			continue
		}

		// Handle message based on type
		switch msg.Type {
		case MessageTypeJoinRoom:
			c.handleJoinRoom(msg.Room)
		case MessageTypeLeaveRoom:
			c.handleLeaveRoom(msg.Room)
		case MessageTypePublish:
			c.handlePublish(msg)
		}

		// Message processed
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

			// Message sent

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// InRoom checks if client is in a specific room
func (c *Client) InRoom(room string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rooms[room]
}

func (c *Client) handleJoinRoom(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.rooms[room] {
		c.rooms[room] = true
		c.hub.JoinRoom(c, room)
	}
}

func (c *Client) handleLeaveRoom(room string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rooms[room] {
		delete(c.rooms, room)
		c.hub.LeaveRoom(c, room)
	}
}

// handlePublish handles message publishing
func (c *Client) handlePublish(msg Message) {
	if msg.Room == "" {
		c.hub.log.Error("Room not specified for publish")
		return
	}

	if !c.InRoom(msg.Room) {
		c.hub.log.Error("Client not in room", zap.String("room", msg.Room))
		return
	}

	c.hub.broadcast <- &Broadcast{
		Room:    msg.Room,
		Message: msg.Payload,
		// Using updated Broadcast struct
	}
}
