package repository

import (
	"context"
	"errors"

	"github.com/linkmeAman/universal-middleware/internal/database"
)

var (
	ErrNotFound      = errors.New("entity not found")
	ErrAlreadyExists = errors.New("entity already exists")
)

// Repository defines common database operations
type Repository interface {
	// Transaction runs the given function in a transaction
	Transaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// BaseRepository provides common functionality for repositories
type BaseRepository struct {
	db database.DB
}

// NewBaseRepository creates a new base repository
func NewBaseRepository(db database.DB) BaseRepository {
	return BaseRepository{db: db}
}

// Transaction wraps a function in a database transaction
func (r *BaseRepository) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}

	// Create a new context with the transaction
	txCtx := context.WithValue(ctx, txKey{}, tx)

	// Execute the function
	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}

	// Commit the transaction
	return tx.Commit(ctx)
}

// txKey is the key type for the transaction context
type txKey struct{}

// GetTx retrieves a transaction from the context
func GetTx(ctx context.Context) (database.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(database.Tx)
	return tx, ok
}

// getQuerier returns either a transaction if one exists in the context,
// or falls back to the database connection
func (r *BaseRepository) getQuerier(ctx context.Context) interface {
	database.DB
	database.Tx
} {
	if tx, ok := GetTx(ctx); ok {
		return tx
	}
	return r.db
}
