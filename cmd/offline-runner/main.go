package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/linkmeAman/universal-middleware/internal/command"
	"github.com/linkmeAman/universal-middleware/internal/command/outbox"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
)

func main() {
	// Setup logger
	log, err := logger.New("offline-runner", "debug")
	if err != nil {
		fmt.Printf("failed to create logger: %v\n", err)
		return
	}

	// In-memory outbox repo
	repo := outbox.NewInMemoryRepository()

	// Mock publisher
	pub := NewMockPublisher()

	// Start outbox processor
	outboxProc := NewOutboxProcessor(repo, pub, ProcessorConfig{BatchSize: 10, PollingInterval: 200 * time.Millisecond}, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	outboxProc.Start(ctx)

	// Setup command processor
	cmdProc := command.NewProcessor(command.ProcessorConfig{MaxWorkers: 5}, log)
	cmdProc.SetValidator(command.NewMockValidator())

	// Register user create handler
	handler := NewUserCreateHandler(repo, log)
	cmdProc.RegisterHandler(handler)

	// Create a demo command
	cmd := command.NewCommand(command.CommandTypeUserCreate, map[string]interface{}{
		"email":    "offline@example.com",
		"username": "offline-user",
		"password": "password123",
	})

	fmt.Println("Processing command:", cmd.ID)
	if err := cmdProc.Process(context.Background(), cmd); err != nil {
		fmt.Println("Command processing failed:", err)
	} else {
		fmt.Println("Command processed, outbox entry created; processor will publish shortly")
	}

	// Wait for publish to occur (short)
	time.Sleep(1 * time.Second)

	// graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigCh:
		fmt.Println("received shutdown signal")
	case <-time.After(500 * time.Millisecond):
		// exit after short wait
	}

	outboxProc.Stop()
	fmt.Println("offline run complete")
}
