package command

import "context"

// CommandValidator defines the interface for command validation
type CommandValidator interface {
	ValidateCommand(ctx context.Context, cmd *Command) error
}
