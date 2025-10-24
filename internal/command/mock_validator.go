package command

import (
	"context"
	"fmt"
)

// MockValidator is a simplified validator for offline testing
type MockValidator struct{}

// NewMockValidator creates a new mock validator
func NewMockValidator() *MockValidator {
	return &MockValidator{}
}

// ValidateCommand performs basic validation on a command
func (v *MockValidator) ValidateCommand(ctx context.Context, cmd *Command) error {
	if cmd.ID == "" {
		return fmt.Errorf("command ID is required")
	}
	if cmd.Type == "" {
		return fmt.Errorf("command type is required")
	}
	if cmd.Payload == nil {
		return fmt.Errorf("command payload is required")
	}

	switch cmd.Type {
	case CommandTypeUserCreate:
		return v.validateUserCreate(cmd.Payload)
	default:
		return nil
	}
}

func (v *MockValidator) validateUserCreate(payload map[string]interface{}) error {
	required := []string{"email", "username", "password"}
	for _, field := range required {
		if val, ok := payload[field]; !ok || val == "" {
			return fmt.Errorf("missing or empty required field: %s", field)
		}
	}
	return nil
}
