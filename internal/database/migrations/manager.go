package migrations

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.uber.org/zap"
)

//go:embed schema/*.sql
var migrationFiles embed.FS

// Manager handles database migrations
type Manager struct {
	migrate *migrate.Migrate
	logger  *logger.Logger
}

// NewManager creates a new migration manager
func NewManager(dsn string, log *logger.Logger) (*Manager, error) {
	// Create driver for embedded files
	d, err := iofs.New(migrationFiles, "schema")
	if err != nil {
		return nil, fmt.Errorf("failed to create migration source: %w", err)
	}

	// Parse database URL
	config, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create migration config: %w", err)
	}

	// Create migrator
	m, err := migrate.NewWithInstance(
		"iofs", d,
		"postgres", config,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return &Manager{
		migrate: m,
		logger:  log,
	}, nil
}

// Up runs all pending migrations
func (m *Manager) Up(ctx context.Context) error {
	start := time.Now()
	m.logger.Info("Running database migrations")

	if err := m.migrate.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	m.logger.Info("Migrations completed",
		zap.Duration("duration", time.Since(start)),
	)
	return nil
}

// Down rolls back all migrations
func (m *Manager) Down(ctx context.Context) error {
	if err := m.migrate.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}
	return nil
}

// Version returns the current migration version
func (m *Manager) Version() (uint, bool, error) {
	return m.migrate.Version()
}

// Close closes the migration manager
func (m *Manager) Close() error {
	return m.migrate.Close()
}
