package postgres

import (
	_ "github.com/jackc/pgx/v5/stdlib" // Register pgx driver
	"github.com/linkmeAman/universal-middleware/internal/database"
	"github.com/linkmeAman/universal-middleware/pkg/config"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
)

// InitFromConfig initializes a database connection from config
func InitFromConfig(cfg *config.Config, log *logger.Logger, m *metrics.Metrics) (*DB, error) {
	opts := database.Options{
		Host:     cfg.Database.Primary.Host,
		Port:     cfg.Database.Primary.Port,
		User:     cfg.Database.Primary.Username,
		Password: cfg.Database.Primary.Password,
		Database: cfg.Database.Primary.Database,
		MaxConns: int32(cfg.Database.Primary.MaxOpenConns),
		MinConns: int32(cfg.Database.Primary.MaxIdleConns),
	}

	return New(opts, log, m)
}
