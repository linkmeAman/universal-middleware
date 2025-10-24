package schemas

import (
	"encoding/json"
	"time"
)

// EventType represents different types of events in the system
type EventType string

const (
	// System events
	EventTypeSystemStartup  EventType = "system.startup"
	EventTypeSystemShutdown EventType = "system.shutdown"

	// User events
	EventTypeUserCreated EventType = "user.created"
	EventTypeUserUpdated EventType = "user.updated"
	EventTypeUserDeleted EventType = "user.deleted"

	// Cache events
	EventTypeCacheInvalidated EventType = "cache.invalidated"
	EventTypeCacheWarmed      EventType = "cache.warmed"

	// Command events
	EventTypeCommandReceived   EventType = "command.received"
	EventTypeCommandProcessed  EventType = "command.processed"
	EventTypeCommandFailed     EventType = "command.failed"
	EventTypeCommandCompleted  EventType = "command.completed"
	EventTypeCommandRollbacked EventType = "command.rollbacked"

	// Message queue events
	EventTypeMessageDeadLettered EventType = "message.dead_lettered"
	EventTypeMessageRetried      EventType = "message.retried"

	// WebSocket events
	EventTypeWSClientConnected    EventType = "ws.client.connected"
	EventTypeWSClientDisconnected EventType = "ws.client.disconnected"
	EventTypeWSMessageSent        EventType = "ws.message.sent"
	EventTypeWSMessageReceived    EventType = "ws.message.received"
)

// Event represents the base event structure
type Event struct {
	ID            string                 `json:"id"`
	Type          EventType              `json:"type"`
	Source        string                 `json:"source"`
	DataVersion   string                 `json:"dataVersion"`
	Time          time.Time              `json:"time"`
	CorrelationID string                 `json:"correlationId,omitempty"`
	CausationID   string                 `json:"causationId,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Marshal converts an event to JSON bytes
func (e *Event) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// Unmarshal converts JSON bytes to an event
func (e *Event) Unmarshal(data []byte) error {
	return json.Unmarshal(data, e)
}

// Command events
type CommandReceivedEvent struct {
	Event
	CommandID   string `json:"commandId"`
	CommandType string `json:"commandType"`
	UserID      string `json:"userId"`
}

type CommandProcessedEvent struct {
	Event
	CommandID    string `json:"commandId"`
	CommandType  string `json:"commandType"`
	ProcessingMS int64  `json:"processingMs"`
}

type CommandFailedEvent struct {
	Event
	CommandID   string `json:"commandId"`
	CommandType string `json:"commandType"`
	Error       string `json:"error"`
	ErrorCode   string `json:"errorCode"`
}

// Cache events
type CacheInvalidatedEvent struct {
	Event
	Key     string `json:"key"`
	Pattern string `json:"pattern,omitempty"`
	Reason  string `json:"reason"`
	CacheID string `json:"cacheId"`
	UserID  string `json:"userId,omitempty"`
}

type CacheWarmedEvent struct {
	Event
	Pattern    string `json:"pattern"`
	KeysWarmed int    `json:"keysWarmed"`
	DurationMS int64  `json:"durationMs"`
	CacheID    string `json:"cacheId"`
}

// WebSocket events
type WSClientEvent struct {
	Event
	ClientID    string `json:"clientId"`
	UserID      string `json:"userId,omitempty"`
	ConnectedAt string `json:"connectedAt,omitempty"`
}

type WSMessageEvent struct {
	Event
	ClientID    string `json:"clientId"`
	UserID      string `json:"userId,omitempty"`
	MessageID   string `json:"messageId"`
	MessageType string `json:"messageType"`
	Room        string `json:"room,omitempty"`
	Size        int    `json:"size"`
}
