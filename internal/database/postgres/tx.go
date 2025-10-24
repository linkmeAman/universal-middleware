package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/linkmeAman/universal-middleware/internal/database"
)

// Tx implements the database.Tx interface
type Tx struct {
	tx pgx.Tx
	db *DB
}

// Commit commits the transaction
func (tx *Tx) Commit(ctx context.Context) error {
	ctx, span := tx.db.startSpan(ctx, "tx.Commit")
	defer span.End()

	err := tx.tx.Commit(ctx)
	if err != nil {
		tx.db.recordError(span, err)
		return fmt.Errorf("commit transaction failed: %w", err)
	}
	return nil
}

// Rollback aborts the transaction
func (tx *Tx) Rollback(ctx context.Context) error {
	ctx, span := tx.db.startSpan(ctx, "tx.Rollback")
	defer span.End()

	err := tx.tx.Rollback(ctx)
	if err != nil {
		tx.db.recordError(span, err)
		return fmt.Errorf("rollback transaction failed: %w", err)
	}
	return nil
}

// Exec executes a query that doesn't return rows within the transaction
func (tx *Tx) Exec(ctx context.Context, sql string, arguments ...interface{}) (database.CommandTag, error) {
	ctx, span := tx.db.startSpan(ctx, "tx.Exec")
	defer span.End()

	tag, err := tx.tx.Exec(ctx, sql, arguments...)
	if err != nil {
		tx.db.recordError(span, err)
		return nil, fmt.Errorf("exec query in transaction failed: %w", err)
	}
	return commandTag{tag}, nil
}

// Query executes a query that returns rows within the transaction
func (tx *Tx) Query(ctx context.Context, sql string, args ...interface{}) (database.Rows, error) {
	ctx, span := tx.db.startSpan(ctx, "tx.Query")
	defer span.End()

	rows, err := tx.tx.Query(ctx, sql, args...)
	if err != nil {
		tx.db.recordError(span, err)
		return nil, fmt.Errorf("query in transaction failed: %w", err)
	}
	return rows, nil
}

// QueryRow executes a query that returns at most one row within the transaction
func (tx *Tx) QueryRow(ctx context.Context, sql string, args ...interface{}) database.Row {
	ctx, span := tx.db.startSpan(ctx, "tx.QueryRow")
	defer span.End()

	return tx.tx.QueryRow(ctx, sql, args...)
}

// CommandTag wraps pgx/v5's pgconn.CommandTag to implement database.CommandTag
type commandTag struct {
	ct pgconn.CommandTag
}

func (t commandTag) RowsAffected() int64 {
	return t.ct.RowsAffected()
}
