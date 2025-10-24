package repository

import (
	"context"
	"testing"
	"time"

	"github.com/linkmeAman/universal-middleware/internal/database"
	"github.com/linkmeAman/universal-middleware/internal/database/postgres"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) database.DB {
	log, err := logger.New("test", "debug")
	require.NoError(t, err)

	opts := database.Options{
		Host:        "localhost",
		Port:        5432,
		User:        "postgres",
		Password:    "postgres",
		Database:    "test_db",
		MaxConns:    5,
		MinConns:    1,
		MaxIdleTime: time.Minute,
		DialTimeout: 5 * time.Second,
	}

	db, err := postgres.New(opts, log, nil)
	require.NoError(t, err)

	// Run migrations
	_, err = db.Exec(context.Background(), `
		DROP TABLE IF EXISTS sessions;
		DROP TABLE IF EXISTS audit_logs;
		DROP TABLE IF EXISTS users;

		CREATE TABLE users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(255) NOT NULL UNIQUE,
			email VARCHAR(255) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			full_name VARCHAR(255),
			role VARCHAR(50) NOT NULL DEFAULT 'user',
			status VARCHAR(50) NOT NULL DEFAULT 'active',
			metadata JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	require.NoError(t, err)

	return db
}

func TestUserRepository(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewUserRepository(db)

	t.Run("Create User", func(t *testing.T) {
		user := &User{
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: "hash",
			FullName:     "Test User",
			Role:         "user",
			Status:       "active",
			Metadata: map[string]interface{}{
				"key": "value",
			},
		}

		err := repo.Create(context.Background(), user)
		require.NoError(t, err)
		require.NotEmpty(t, user.ID)
		require.NotZero(t, user.CreatedAt)
		require.NotZero(t, user.UpdatedAt)
	})

	t.Run("Get User By ID", func(t *testing.T) {
		user := &User{
			Username:     "getuser",
			Email:        "get@example.com",
			PasswordHash: "hash",
			FullName:     "Get User",
		}

		err := repo.Create(context.Background(), user)
		require.NoError(t, err)

		found, err := repo.GetByID(context.Background(), user.ID)
		require.NoError(t, err)
		require.Equal(t, user.Username, found.Username)
		require.Equal(t, user.Email, found.Email)
	})

	t.Run("Update User", func(t *testing.T) {
		user := &User{
			Username:     "updateuser",
			Email:        "update@example.com",
			PasswordHash: "hash",
			FullName:     "Update User",
		}

		err := repo.Create(context.Background(), user)
		require.NoError(t, err)

		user.FullName = "Updated Name"
		user.Email = "updated@example.com"

		err = repo.Update(context.Background(), user)
		require.NoError(t, err)

		found, err := repo.GetByID(context.Background(), user.ID)
		require.NoError(t, err)
		require.Equal(t, "Updated Name", found.FullName)
		require.Equal(t, "updated@example.com", found.Email)
	})

	t.Run("Delete User", func(t *testing.T) {
		user := &User{
			Username:     "deleteuser",
			Email:        "delete@example.com",
			PasswordHash: "hash",
			FullName:     "Delete User",
		}

		err := repo.Create(context.Background(), user)
		require.NoError(t, err)

		err = repo.Delete(context.Background(), user.ID)
		require.NoError(t, err)

		_, err = repo.GetByID(context.Background(), user.ID)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("Find By Username", func(t *testing.T) {
		user := &User{
			Username:     "findbyuser",
			Email:        "findbyuser@example.com",
			PasswordHash: "hash",
			FullName:     "Find User",
		}

		err := repo.Create(context.Background(), user)
		require.NoError(t, err)

		found, err := repo.FindByUsername(context.Background(), user.Username)
		require.NoError(t, err)
		require.Equal(t, user.ID, found.ID)
		require.Equal(t, user.Email, found.Email)
	})

	t.Run("Find By Email", func(t *testing.T) {
		user := &User{
			Username:     "findbyemail",
			Email:        "findbyemail@example.com",
			PasswordHash: "hash",
			FullName:     "Find Email User",
		}

		err := repo.Create(context.Background(), user)
		require.NoError(t, err)

		found, err := repo.FindByEmail(context.Background(), user.Email)
		require.NoError(t, err)
		require.Equal(t, user.ID, found.ID)
		require.Equal(t, user.Username, found.Username)
	})

	t.Run("Transaction", func(t *testing.T) {
		user1 := &User{
			Username:     "txuser1",
			Email:        "tx1@example.com",
			PasswordHash: "hash",
		}
		user2 := &User{
			Username:     "txuser2",
			Email:        "tx2@example.com",
			PasswordHash: "hash",
		}

		err := repo.Transaction(context.Background(), func(ctx context.Context) error {
			if err := repo.Create(ctx, user1); err != nil {
				return err
			}
			if err := repo.Create(ctx, user2); err != nil {
				return err
			}
			return nil
		})
		require.NoError(t, err)

		// Verify both users were created
		_, err = repo.GetByID(context.Background(), user1.ID)
		require.NoError(t, err)
		_, err = repo.GetByID(context.Background(), user2.ID)
		require.NoError(t, err)
	})
}
