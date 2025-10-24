package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// DB defines the interface for database operations
type DB interface {
	// Core operations
	Exec(ctx context.Context, sql string, arguments ...interface{}) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) Row

	// Transaction management
	Begin(ctx context.Context) (Tx, error)
	BeginTx(ctx context.Context, txOptions TxOptions) (Tx, error)

	// Connection management
	Close()
	Ping(ctx context.Context) error

	// Stats and metrics
	Stats() *Stats
}

// Tx represents a database transaction
type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error

	// Transaction operations
	Exec(ctx context.Context, sql string, arguments ...interface{}) (CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) Row
}

// Row represents a single database row
type Row interface {
	Scan(dest ...interface{}) error
}

// Rows represents multiple database rows
type Rows interface {
	Close()
	Err() error
	Next() bool
	Scan(dest ...interface{}) error
}

// CommandTag represents the results of an Exec command
type CommandTag interface {
	RowsAffected() int64
}

// TxOptions represents transaction options
type TxOptions struct {
	IsolationLevel pgx.Tx
	ReadOnly       bool
	Deferrable     bool
}

// Stats provides database statistics
type Stats struct {
	MaxOpenConnections int
	OpenConnections    int
	InUse              int
	Idle               int
	WaitCount          int64
	WaitDuration       time.Duration
	MaxIdleClosed      int64
	MaxLifetimeClosed  int64
}

// Options contains database configuration options
type Options struct {
	Host         string
	Port         int
	User         string
	Password     string
	Database     string
	MaxConns     int32
	MinConns     int32
	MaxIdleTime  time.Duration
	DialTimeout  time.Duration
	WriteTimeout time.Duration
	ReadTimeout  time.Duration
}
