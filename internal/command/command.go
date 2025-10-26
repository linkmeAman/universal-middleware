package command

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Status represents the command status
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusRetrying   Status = "retrying"
	StatusCancelled  Status = "cancelled"
)

// Priority represents command processing priority
type Priority int

const (
	PriorityLow      Priority = 1
	PriorityNormal   Priority = 2
	PriorityHigh     Priority = 3
	PriorityCritical Priority = 4
)

// Command represents a base command structure
type Command struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Priority       Priority               `json:"priority"`
	Status         Status                 `json:"status"`
	Payload        map[string]interface{} `json:"payload"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	ErrorDetails   *ErrorDetails          `json:"errorDetails,omitempty"`
	CreatedAt      time.Time              `json:"createdAt"`
	UpdatedAt      time.Time              `json:"updatedAt"`
	ScheduledFor   *time.Time             `json:"scheduledFor,omitempty"`
	ProcessedAt    *time.Time             `json:"processedAt,omitempty"`
	CompletedAt    *time.Time             `json:"completedAt,omitempty"`
	RetryCount     int                    `json:"retryCount"`
	MaxRetries     int                    `json:"maxRetries"`
	RetryBackoff   time.Duration          `json:"retryBackoff"`
	TimeoutAfter   time.Duration          `json:"timeoutAfter"`
	CorrelationID  string                 `json:"correlationId,omitempty"`
	UserID         string                 `json:"userId,omitempty"`
	IdempotencyKey string                 `json:"idempotencyKey,omitempty"`
	EntityID       string                 `json:"entityId,omitempty"`
	Error          string                 `json:"error,omitempty"`
}

// ErrorDetails holds information about command failures
type ErrorDetails struct {
	Code       string    `json:"code"`
	Message    string    `json:"message"`
	Details    string    `json:"details,omitempty"`
	OccurredAt time.Time `json:"occurredAt"`
}

// NewCommand creates a new command with default values
func NewCommand(cmdType string, payload map[string]interface{}) *Command {
	now := time.Now()
	return &Command{
		ID:           uuid.New().String(),
		Type:         cmdType,
		Priority:     PriorityNormal,
		Status:       StatusPending,
		Payload:      payload,
		Metadata:     make(map[string]interface{}),
		CreatedAt:    now,
		UpdatedAt:    now,
		MaxRetries:   3,
		RetryBackoff: 5 * time.Second,
		TimeoutAfter: 30 * time.Second,
	}
}

// Marshal serializes the command to JSON
func (c *Command) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

// Unmarshal deserializes the command from JSON
func (c *Command) Unmarshal(data []byte) error {
	return json.Unmarshal(data, c)
}

// SetError sets error details for the command
func (c *Command) SetError(code string, message string, details string) {
	c.ErrorDetails = &ErrorDetails{
		Code:       code,
		Message:    message,
		Details:    details,
		OccurredAt: time.Now(),
	}
}

// IsRetryable checks if the command can be retried
func (c *Command) IsRetryable() bool {
	return c.Status == StatusFailed && c.RetryCount < c.MaxRetries
}

// HasTimedOut checks if the command has timed out
func (c *Command) HasTimedOut() bool {
	if c.TimeoutAfter <= 0 {
		return false
	}
	if c.ProcessedAt == nil {
		return false
	}
	return time.Since(*c.ProcessedAt) > c.TimeoutAfter
}

// ShouldProcess checks if the command should be processed now
func (c *Command) ShouldProcess() bool {
	if c.Status != StatusPending && c.Status != StatusRetrying {
		return false
	}
	if c.ScheduledFor != nil && time.Now().Before(*c.ScheduledFor) {
		return false
	}
	return true
}

// Common command types
const (
	CommandTypeUserCreate      = "user.create"
	CommandTypeUserUpdate      = "user.update"
	CommandTypeUserDelete      = "user.delete"
	CommandTypeEmailSend       = "email.send"
	CommandTypePaymentProcess  = "payment.process"
	CommandTypeOrderCreate     = "order.create"
	CommandTypeOrderCancel     = "order.cancel"
	CommandTypeCacheInvalidate = "cache.invalidate"
	CommandTypeCacheWarmup     = "cache.warmup"
)
