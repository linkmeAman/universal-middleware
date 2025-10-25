package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/linkmeAman/universal-middleware/internal/database"
)

// TestData represents the structure of test fixture data
type TestData struct {
	Commands []map[string]interface{} `json:"commands"`
	Users    []map[string]interface{} `json:"users"`
}

// LoadTestData loads test fixtures from JSON files
func LoadTestData() (*TestData, error) {
	data := &TestData{}

	// Load commands
	if err := loadJSONFile("fixtures/commands.json", &data.Commands); err != nil {
		return nil, fmt.Errorf("failed to load commands: %w", err)
	}

	// Load users
	if err := loadJSONFile("fixtures/users.json", &data.Users); err != nil {
		return nil, fmt.Errorf("failed to load users: %w", err)
	}

	return data, nil
}

// loadJSONFile loads and parses a JSON file
func loadJSONFile(path string, v interface{}) error {
	fullPath := filepath.Join("test/integration", path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// SetupTestDB prepares the test database with fixtures
func SetupTestDB(ctx context.Context, db *database.DB) error {
	// Clear existing data
	if err := clearTestData(ctx, db); err != nil {
		return err
	}

	// Load test fixtures
	data, err := LoadTestData()
	if err != nil {
		return err
	}

	// Insert test data
	if err := insertTestData(ctx, db, data); err != nil {
		return err
	}

	return nil
}

// clearTestData removes existing test data
func clearTestData(ctx context.Context, db *database.DB) error {
	tables := []string{"users", "commands", "outbox_messages"}
	for _, table := range tables {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)); err != nil {
			return fmt.Errorf("failed to truncate %s: %w", table, err)
		}
	}
	return nil
}

// insertTestData populates the database with test fixtures
func insertTestData(ctx context.Context, db *database.DB, data *TestData) error {
	// Insert users
	for _, user := range data.Users {
		if err := insertUser(ctx, db, user); err != nil {
			return err
		}
	}

	// Insert commands
	for _, cmd := range data.Commands {
		if err := insertCommand(ctx, db, cmd); err != nil {
			return err
		}
	}

	return nil
}

// insertUser inserts a test user
func insertUser(ctx context.Context, db *database.DB, user map[string]interface{}) error {
	query := `
		INSERT INTO users (id, username, email, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
	`
	_, err := db.ExecContext(ctx, query,
		user["id"],
		user["username"],
		user["email"],
	)
	return err
}

// insertCommand inserts a test command
func insertCommand(ctx context.Context, db *database.DB, cmd map[string]interface{}) error {
	query := `
		INSERT INTO commands (id, type, payload, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
	`
	_, err := db.ExecContext(ctx, query,
		cmd["id"],
		cmd["type"],
		cmd["payload"],
		cmd["status"],
	)
	return err
}
