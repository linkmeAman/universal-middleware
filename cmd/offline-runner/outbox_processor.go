package main

import (
	"context"
	"time"

	"github.com/linkmeAman/universal-middleware/internal/command/outbox"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	// "go.uber.org/zap"
)

// OutboxProcessor polls an in-memory repository and publishes messages using MockPublisher
type OutboxProcessor struct {
	repo *outbox.InMemoryRepository
	pub  *MockPublisher
	cfg  ProcessorConfig
	log  *logger.Logger
	quit chan struct{}
}

// ProcessorConfig config for offline processor
type ProcessorConfig struct {
	BatchSize       int
	PollingInterval time.Duration
}

func NewOutboxProcessor(repo *outbox.InMemoryRepository, pub *MockPublisher, cfg ProcessorConfig, log *logger.Logger) *OutboxProcessor {
	return &OutboxProcessor{repo: repo, pub: pub, cfg: cfg, log: log, quit: make(chan struct{})}
}

func (p *OutboxProcessor) Start(ctx context.Context) {
	ticker := time.NewTicker(p.cfg.PollingInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.quit:
				return
			case <-ticker.C:
				msgs, _ := p.repo.GetPendingMessages(ctx, p.cfg.BatchSize)
				for _, m := range msgs {
					_ = p.pub.Publish(ctx, m.Topic, m.ID, m.Payload)
					_ = p.repo.MarkAsPublished(ctx, m.ID)
				}
			}
		}
	}()
}

func (p *OutboxProcessor) Stop() {
	close(p.quit)
}
