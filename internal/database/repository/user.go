package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
)

// User represents a user in the system
type User struct {
	ID           uuid.UUID
	Username     string
	Email        string
	PasswordHash string
	FullName     string
	Role         string
	Status       string
	Metadata     map[string]interface{}
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserRepository handles user-related database operations
type UserRepository struct {
	BaseRepository
}

// NewUserRepository creates a new user repository
func NewUserRepository(db DB) *UserRepository {
	return &UserRepository{
		BaseRepository: NewBaseRepository(db),
	}
}

// Create inserts a new user
func (r *UserRepository) Create(ctx context.Context, user *User) error {
	q := `
		INSERT INTO users (username, email, password_hash, full_name, role, status, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	row := r.getQuerier(ctx).QueryRow(ctx, q,
		user.Username,
		user.Email,
		user.PasswordHash,
		user.FullName,
		user.Role,
		user.Status,
		user.Metadata,
	)

	err := row.Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if pgErr, ok := err.(*pgx.Error); ok && pgErr.Code == "23505" {
			return ErrAlreadyExists
		}
		return err
	}

	return nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	user := &User{}
	q := `
		SELECT id, username, email, password_hash, full_name, role, status, metadata, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	err := r.getQuerier(ctx).QueryRow(ctx, q, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.Role,
		&user.Status,
		&user.Metadata,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return user, nil
}

// Update updates user information
func (r *UserRepository) Update(ctx context.Context, user *User) error {
	q := `
		UPDATE users
		SET username = $2, email = $3, full_name = $4, role = $5, status = $6, metadata = $7, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.getQuerier(ctx).QueryRow(ctx, q,
		user.ID,
		user.Username,
		user.Email,
		user.FullName,
		user.Role,
		user.Status,
		user.Metadata,
	).Scan(&user.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if pgErr, ok := err.(*pgx.Error); ok && pgErr.Code == "23505" {
			return ErrAlreadyExists
		}
		return err
	}

	return nil
}

// Delete removes a user
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	q := `DELETE FROM users WHERE id = $1`

	result, err := r.getQuerier(ctx).Exec(ctx, q, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	return nil
}

// FindByUsername retrieves a user by username
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	user := &User{}
	q := `
		SELECT id, username, email, password_hash, full_name, role, status, metadata, created_at, updated_at
		FROM users
		WHERE username = $1
	`

	err := r.getQuerier(ctx).QueryRow(ctx, q, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.Role,
		&user.Status,
		&user.Metadata,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return user, nil
}

// FindByEmail retrieves a user by email
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	q := `
		SELECT id, username, email, password_hash, full_name, role, status, metadata, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	err := r.getQuerier(ctx).QueryRow(ctx, q, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.Role,
		&user.Status,
		&user.Metadata,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return user, nil
}
