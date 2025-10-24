package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/linkmeAman/universal-middleware/internal/database"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// DB implements the database.DB interface for PostgreSQL
type DB struct {
	pool    *pgxpool.Pool
	logger  *logger.Logger
	metrics *metrics.Metrics
	tracer  trace.Tracer
}

// New creates a new PostgreSQL database connection pool
func New(opts database.Options, log *logger.Logger, m *metrics.Metrics) (*DB, error) {
	config, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		opts.User,
		opts.Password,
		opts.Host,
		opts.Port,
		opts.Database,
	))
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Configure connection pool
	config.MaxConns = opts.MaxConns
	config.MinConns = opts.MinConns
	config.MaxConnLifetime = opts.MaxIdleTime
	config.ConnConfig.ConnectTimeout = opts.DialTimeout

	// Create connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &DB{
		pool:    pool,
		logger:  log,
		metrics: m,
		tracer:  otel.GetTracerProvider().Tracer("postgres-db"),
	}, nil
}

// startSpan starts a new trace span for database operations
func (db *DB) startSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return db.tracer.Start(ctx, name,
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", name),
		),
	)
}

// Exec executes a query that doesn't return rows
func (db *DB) Exec(ctx context.Context, sql string, arguments ...interface{}) (database.CommandTag, error) {
	ctx, span := db.startSpan(ctx, "db.Exec")
	defer span.End()

	start := time.Now()
	tag, err := db.pool.Exec(ctx, sql, arguments...)
	if err != nil {
		db.recordError(span, err)
		return nil, fmt.Errorf("exec query failed: %w", err)
	}

	db.recordMetrics("exec", time.Since(start))
	return commandTag{tag}, nil
}

// Query executes a query that returns rows
func (db *DB) Query(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
	ctx, span := db.startSpan(ctx, "db.Query")
	defer span.End()

	start := time.Now()
	rows, err := db.pool.Query(ctx, sql, args...)
	if err != nil {
		db.recordError(span, err)
		return nil, fmt.Errorf("query failed: %w", err)
	}

	db.recordMetrics("query", time.Since(start))
	return rows, nil
}

// QueryRow executes a query that returns at most one row
func (db *DB) QueryRow(ctx context.Context, sql string, args ...interface{}) database.Row {
	ctx, span := db.startSpan(ctx, "db.QueryRow")
	defer span.End()

	start := time.Now()
	row := db.pool.QueryRow(ctx, sql, args...)
	db.recordMetrics("queryRow", time.Since(start))
	return row
}

// Begin starts a new transaction
func (db *DB) Begin(ctx context.Context) (database.Tx, error) {
	return db.BeginTx(ctx, database.TxOptions{})
}

// BeginTx starts a new transaction with options
func (db *DB) BeginTx(ctx context.Context, opts database.TxOptions) (database.Tx, error) {
	ctx, span := db.startSpan(ctx, "db.BeginTx")
	defer span.End()

	tx, err := db.pool.Begin(ctx)
	if err != nil {
		db.recordError(span, err)
		return nil, fmt.Errorf("begin transaction failed: %w", err)
	}

	return &Tx{tx: tx, db: db}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	db.pool.Close()
}

// Ping verifies a connection to the database is still alive
func (db *DB) Ping(ctx context.Context) error {
	ctx, span := db.startSpan(ctx, "db.Ping")
	defer span.End()

	return db.pool.Ping(ctx)
}

// Stats returns connection pool statistics
func (db *DB) Stats() *database.Stats {
	stats := db.pool.Stat()
	return &database.Stats{
		MaxOpenConnections: int(stats.MaxConns()),
		OpenConnections:    int(stats.TotalConns()),
		InUse:              int(stats.AcquiredConns()),
		Idle:               int(stats.IdleConns()),
	}
}

// recordError records an error in tracing
func (db *DB) recordError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetAttributes(attribute.Bool("error", true))
}

// recordMetrics records query execution metrics
func (db *DB) recordMetrics(op string, duration time.Duration) {
	if db.metrics != nil {
		db.metrics.DBQueryDuration.WithLabelValues(op).Observe(duration.Seconds())
	}
}
