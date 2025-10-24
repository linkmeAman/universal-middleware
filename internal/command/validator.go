package command

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Validator handles command validation
type Validator struct {
	validate *validator.Validate
	tracer   trace.Tracer
}

// NewValidator creates a new command validator
func NewValidator() *Validator {
	return &Validator{
		validate: validator.New(),
		tracer:   trace.NewNoopTracerProvider().Tracer("command-validator"),
	}
}

// ValidateCommand performs validation on a command
func (v *Validator) ValidateCommand(ctx context.Context, cmd *Command) error {
	ctx, span := v.tracer.Start(ctx, "validate_command",
		trace.WithAttributes(
			attribute.String("command.id", cmd.ID),
			attribute.String("command.type", cmd.Type),
		),
	)
	defer span.End()

	// Basic validation
	if cmd.ID == "" {
		return fmt.Errorf("command ID is required")
	}
	if cmd.Type == "" {
		return fmt.Errorf("command type is required")
	}
	if cmd.Payload == nil {
		return fmt.Errorf("command payload is required")
	}

	// Type-specific validation
	switch cmd.Type {
	case CommandTypeUserCreate:
		return v.validateUserCreate(cmd.Payload)
	case CommandTypeUserUpdate:
		return v.validateUserUpdate(cmd.Payload)
	case CommandTypeEmailSend:
		return v.validateEmailSend(cmd.Payload)
	case CommandTypePaymentProcess:
		return v.validatePaymentProcess(cmd.Payload)
	case CommandTypeOrderCreate:
		return v.validateOrderCreate(cmd.Payload)
	default:
		return v.validateGenericCommand(cmd)
	}
}

func (v *Validator) validateUserCreate(payload map[string]interface{}) error {
	required := []string{"email", "username", "password"}
	for _, field := range required {
		if _, ok := payload[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	email, ok := payload["email"].(string)
	if !ok {
		return fmt.Errorf("email must be a string")
	}
	if err := v.validate.Var(email, "required,email"); err != nil {
		return fmt.Errorf("invalid email: %w", err)
	}

	username, ok := payload["username"].(string)
	if !ok {
		return fmt.Errorf("username must be a string")
	}
	if err := v.validate.Var(username, "required,alphanum,min=3,max=30"); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}

	password, ok := payload["password"].(string)
	if !ok {
		return fmt.Errorf("password must be a string")
	}
	if err := v.validate.Var(password, "required,min=8"); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	return nil
}

func (v *Validator) validateUserUpdate(payload map[string]interface{}) error {
	if _, ok := payload["id"]; !ok {
		return fmt.Errorf("user ID is required for update")
	}

	// Optional fields validation
	if email, ok := payload["email"].(string); ok {
		if err := v.validate.Var(email, "email"); err != nil {
			return fmt.Errorf("invalid email: %w", err)
		}
	}

	if username, ok := payload["username"].(string); ok {
		if err := v.validate.Var(username, "alphanum,min=3,max=30"); err != nil {
			return fmt.Errorf("invalid username: %w", err)
		}
	}

	return nil
}

func (v *Validator) validateEmailSend(payload map[string]interface{}) error {
	required := []string{"to", "subject", "body"}
	for _, field := range required {
		if _, ok := payload[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	to, ok := payload["to"].(string)
	if !ok {
		return fmt.Errorf("'to' field must be a string")
	}
	if err := v.validate.Var(to, "required,email"); err != nil {
		return fmt.Errorf("invalid recipient email: %w", err)
	}

	subject, ok := payload["subject"].(string)
	if !ok {
		return fmt.Errorf("subject must be a string")
	}
	if err := v.validate.Var(subject, "required,min=1,max=255"); err != nil {
		return fmt.Errorf("invalid subject: %w", err)
	}

	return nil
}

func (v *Validator) validatePaymentProcess(payload map[string]interface{}) error {
	required := []string{"amount", "currency", "paymentMethod"}
	for _, field := range required {
		if _, ok := payload[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	amount, ok := payload["amount"].(float64)
	if !ok {
		return fmt.Errorf("amount must be a number")
	}
	if err := v.validate.Var(amount, "required,gt=0"); err != nil {
		return fmt.Errorf("invalid amount: %w", err)
	}

	currency, ok := payload["currency"].(string)
	if !ok {
		return fmt.Errorf("currency must be a string")
	}
	if err := v.validate.Var(currency, "required,iso4217"); err != nil {
		return fmt.Errorf("invalid currency code: %w", err)
	}

	return nil
}

func (v *Validator) validateOrderCreate(payload map[string]interface{}) error {
	required := []string{"userId", "items"}
	for _, field := range required {
		if _, ok := payload[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	items, ok := payload["items"].([]interface{})
	if !ok {
		return fmt.Errorf("items must be an array")
	}
	if len(items) == 0 {
		return fmt.Errorf("order must contain at least one item")
	}

	for i, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("item %d is not a valid object", i)
		}
		if err := v.validateOrderItem(itemMap); err != nil {
			return fmt.Errorf("invalid item %d: %w", i, err)
		}
	}

	return nil
}

func (v *Validator) validateOrderItem(item map[string]interface{}) error {
	required := []string{"productId", "quantity"}
	for _, field := range required {
		if _, ok := item[field]; !ok {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	quantity, ok := item["quantity"].(float64)
	if !ok {
		return fmt.Errorf("quantity must be a number")
	}
	if err := v.validate.Var(quantity, "required,gt=0"); err != nil {
		return fmt.Errorf("invalid quantity: %w", err)
	}

	return nil
}

func (v *Validator) validateGenericCommand(cmd *Command) error {
	// Basic structure validation for any command type
	if cmd.Priority < PriorityLow || cmd.Priority > PriorityCritical {
		return fmt.Errorf("invalid priority value")
	}

	if cmd.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	if cmd.RetryBackoff < 0 {
		return fmt.Errorf("retry backoff cannot be negative")
	}

	if cmd.TimeoutAfter < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}

	return nil
}
