package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/linkmeAman/universal-middleware/internal/database"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestDatabase(t *testing.T) {
	log, err := logger.New("test", "debug")
	require.NoError(t, err)

	// Test database connection
	opts := database.Options{
		Host:        "localhost",
		Port:        5432,
		User:        "postgres",
		Password:    "postgres",
		Database:    "test_db",
		MaxConns:    5,
		MinConns:    1,
		MaxIdleTime: 5 * time.Minute,
		DialTimeout: 5 * time.Second,
	}

	db, err := New(opts, log, nil)
	require.NoError(t, err)
	defer db.Close()

	// Test ping
	err = db.Ping(context.Background())
	require.NoError(t, err)

	// Test basic operations
	t.Run("Basic Operations", func(t *testing.T) {
		// Create test table
		_, err := db.Exec(context.Background(), `
			CREATE TABLE IF NOT EXISTS test_users (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL,
				created_at TIMESTAMPTZ DEFAULT NOW()
			)
		`)
		require.NoError(t, err)

		// Insert data
		tag, err := db.Exec(context.Background(),
			"INSERT INTO test_users (name) VALUES ($1) RETURNING id",
			"test_user",
		)
		require.NoError(t, err)
		require.Equal(t, int64(1), tag.RowsAffected())

		// Query data
		var name string
		err = db.QueryRow(context.Background(),
			"SELECT name FROM test_users WHERE id = 1",
		).Scan(&name)
		require.NoError(t, err)
		require.Equal(t, "test_user", name)

		// Clean up
		_, err = db.Exec(context.Background(), "DROP TABLE test_users")
		require.NoError(t, err)
	})

	// Test transactions
	t.Run("Transactions", func(t *testing.T) {
		// Create test table
		_, err := db.Exec(context.Background(), `
			CREATE TABLE IF NOT EXISTS test_accounts (
				id SERIAL PRIMARY KEY,
				balance INTEGER NOT NULL
			)
		`)
		require.NoError(t, err)

		// Start transaction
		tx, err := db.Begin(context.Background())
		require.NoError(t, err)

		// Insert test data
		_, err = tx.Exec(context.Background(),
			"INSERT INTO test_accounts (balance) VALUES ($1)",
			1000,
		)
		require.NoError(t, err)

		// Update balance
		_, err = tx.Exec(context.Background(),
			"UPDATE test_accounts SET balance = balance - $1 WHERE id = 1",
			500,
		)
		require.NoError(t, err)

		// Commit transaction
		err = tx.Commit(context.Background())
		require.NoError(t, err)

		// Verify balance
		var balance int
		err = db.QueryRow(context.Background(),
			"SELECT balance FROM test_accounts WHERE id = 1",
		).Scan(&balance)
		require.NoError(t, err)
		require.Equal(t, 500, balance)

		// Clean up
		_, err = db.Exec(context.Background(), "DROP TABLE test_accounts")
		require.NoError(t, err)
	})

	// Test rollback
	t.Run("Rollback", func(t *testing.T) {
		// Create test table
		_, err := db.Exec(context.Background(), `
			CREATE TABLE IF NOT EXISTS test_items (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL
			)
		`)
		require.NoError(t, err)

		// Start transaction
		tx, err := db.Begin(context.Background())
		require.NoError(t, err)

		// Insert test data
		_, err = tx.Exec(context.Background(),
			"INSERT INTO test_items (name) VALUES ($1)",
			"test_item",
		)
		require.NoError(t, err)

		// Rollback transaction
		err = tx.Rollback(context.Background())
		require.NoError(t, err)

		// Verify no data was inserted
		var count int
		err = db.QueryRow(context.Background(),
			"SELECT COUNT(*) FROM test_items",
		).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		// Clean up
		_, err = db.Exec(context.Background(), "DROP TABLE test_items")
		require.NoError(t, err)
	})
}
