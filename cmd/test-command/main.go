package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IBM/sarama"
	_ "github.com/lib/pq"
	"github.com/linkmeAman/universal-middleware/internal/command"
	"github.com/linkmeAman/universal-middleware/internal/command/outbox"
	"github.com/linkmeAman/universal-middleware/internal/events/publisher"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.uber.org/zap"
)

// UserCreateHandler handles user creation commands
type UserCreateHandler struct {
	outboxRepo *outbox.Repository
	log        *logger.Logger
}

func (h *UserCreateHandler) HandleCommand(ctx context.Context, cmd *command.Command) error {
	if cmd.Type != command.CommandTypeUserCreate {
		return fmt.Errorf("invalid command type for handler: %s", cmd.Type)
	}

	// Create user event for outbox
	event, err := outbox.CreateMessage(
		"user",
		cmd.ID,
		"user.created",
		cmd.Payload,
	)
	if err != nil {
		return fmt.Errorf("failed to create outbox message: %w", err)
	}
	event.Topic = "users"

	// Save to outbox
	if err := h.outboxRepo.Save(ctx, event); err != nil {
		return fmt.Errorf("failed to save to outbox: %w", err)
	}

	h.log.Info("User creation event saved to outbox",
		zap.String("command_id", cmd.ID),
		zap.String("user_id", cmd.ID),
	)

	return nil
}

func (h *UserCreateHandler) CanHandle(cmdType string) bool {
	return cmdType == command.CommandTypeUserCreate
}

func main() {
	// Initialize logger
	log, err := logger.NewLogger(logger.Config{
		Development: true,
		Level:       "debug",
	})
	if err != nil {
		log.Fatal("Failed to create logger", zap.Error(err))
	}

	// Initialize Kafka producer
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true

	kafkaProducer, err := sarama.NewSyncProducer([]string{"localhost:9092"}, config)
	if err != nil {
		log.Fatal("Failed to create Kafka producer", zap.Error(err))
	}
	defer kafkaProducer.Close()

	// Initialize publisher
	pub, err := publisher.NewProducer(publisher.ProducerConfig{
		Brokers:      []string{"localhost:9092"},
		RequiredAcks: sarama.WaitForAll,
		MaxRetries:   5,
	}, log)
	if err != nil {
		log.Fatal("Failed to create publisher", zap.Error(err))
	}
	defer pub.Close()

	// Initialize database connection
	db, err := sql.Open("postgres", "postgres://localhost:5432/middleware?sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// Create outbox repository
	outboxRepo := outbox.NewRepository(db, log)

	// Create outbox processor
	outboxProcessor := outbox.NewProcessor(
		outbox.DefaultConfig(),
		outboxRepo,
		pub,
		log,
	)

	// Create command processor
	cmdProcessor := command.NewProcessor(command.ProcessorConfig{
		MaxWorkers: 10,
	}, log)

	// Register command handler
	cmdProcessor.RegisterHandler(&UserCreateHandler{
		outboxRepo: outboxRepo,
		log:        log,
	})

	// Start outbox processor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := outboxProcessor.Start(ctx); err != nil {
		log.Fatal("Failed to start outbox processor", zap.Error(err))
	}

	// Create and process a test command
	cmd := command.NewCommand(command.CommandTypeUserCreate, map[string]interface{}{
		"email":    "test@example.com",
		"username": "testuser",
		"password": "securepassword123",
	})

	log.Info("Processing test command", zap.String("command_id", cmd.ID))

	if err := cmdProcessor.Process(ctx, cmd); err != nil {
		log.Error("Failed to process command", zap.Error(err))
	} else {
		log.Info("Command processed successfully", zap.String("command_id", cmd.ID))
	}

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Info("Shutting down...")

	// Allow some time for cleanup
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	// Stop processors
	cancel()

	<-shutdownCtx.Done()
	log.Info("Shutdown complete")
}
