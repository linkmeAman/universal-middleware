package main

import (
	"context"
	"fmt"

	"github.com/linkmeAman/universal-middleware/internal/command"
	"github.com/linkmeAman/universal-middleware/internal/command/outbox"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.uber.org/zap"
)

// UserCreateHandler handles user.create commands and writes outbox messages
type UserCreateHandler struct {
	repo *outbox.InMemoryRepository
	log  *logger.Logger
}

func NewUserCreateHandler(repo *outbox.InMemoryRepository, log *logger.Logger) *UserCreateHandler {
	return &UserCreateHandler{repo: repo, log: log}
}

func (h *UserCreateHandler) HandleCommand(ctx context.Context, cmd *command.Command) error {
	if cmd.Type != command.CommandTypeUserCreate {
		return fmt.Errorf("unsupported command type: %s", cmd.Type)
	}

	event, err := outbox.CreateMessage("user", cmd.ID, "user.created", cmd.Payload)
	if err != nil {
		return err
	}
	event.Topic = "users"

	if err := h.repo.Save(ctx, event); err != nil {
		return err
	}

	h.log.Info("Saved outbox event for user.create",
		zap.String("command_id", cmd.ID),
	)
	return nil
}

func (h *UserCreateHandler) CanHandle(cmdType string) bool {
	return cmdType == command.CommandTypeUserCreate
}
