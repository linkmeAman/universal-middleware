package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Config represents the environment configuration
type Config struct {
	// Session configuration
	SessionSecret string

	// OAuth2 configuration
	OAuth2ProviderURL  string
	OAuth2ClientID     string
	OAuth2ClientSecret string
	OAuth2RedirectURL  string

	// Domain configuration
	Domain string

	// JWT configuration
	JWTSecret string
}

// LoadConfig loads environment variables from .env file and environment
func LoadConfig() (*Config, error) {
	// Try to load .env file if it exists
	if envFile := findEnvFile(); envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			return nil, fmt.Errorf("error loading .env file: %v", err)
		}
	}

	// Required environment variables
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}

	oauth2ProviderURL := os.Getenv("OAUTH2_PROVIDER_URL")
	if oauth2ProviderURL == "" {
		return nil, fmt.Errorf("OAUTH2_PROVIDER_URL is required")
	}

	oauth2ClientID := os.Getenv("OAUTH2_CLIENT_ID")
	if oauth2ClientID == "" {
		return nil, fmt.Errorf("OAUTH2_CLIENT_ID is required")
	}

	oauth2ClientSecret := os.Getenv("OAUTH2_CLIENT_SECRET")
	if oauth2ClientSecret == "" {
		return nil, fmt.Errorf("OAUTH2_CLIENT_SECRET is required")
	}

	oauth2RedirectURL := os.Getenv("OAUTH2_REDIRECT_URL")
	if oauth2RedirectURL == "" {
		return nil, fmt.Errorf("OAUTH2_REDIRECT_URL is required")
	}

	// Optional environment variables with defaults
	domain := os.Getenv("DOMAIN")
	if domain == "" {
		domain = "localhost"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return &Config{
		SessionSecret:      sessionSecret,
		OAuth2ProviderURL:  oauth2ProviderURL,
		OAuth2ClientID:     oauth2ClientID,
		OAuth2ClientSecret: oauth2ClientSecret,
		OAuth2RedirectURL:  oauth2RedirectURL,
		Domain:             domain,
		JWTSecret:          jwtSecret,
	}, nil
}

// findEnvFile looks for a .env file in the current directory and its parent directories
func findEnvFile() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		envPath := filepath.Join(dir, ".env")
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}

		parentDir := filepath.Dir(dir)
		if parentDir == dir {
			break
		}
		dir = parentDir
	}

	return ""
}

// LoadTestConfig loads test environment variables from .env.test file
func LoadTestConfig() (*Config, error) {
	// Try to load .env.test file
	if err := godotenv.Load(".env.test"); err != nil {
		return nil, fmt.Errorf("error loading .env.test file: %v", err)
	}

	return LoadConfig()
}

// SetTestEnv sets an environment variable for testing
func SetTestEnv(key, value string) error {
	return os.Setenv(key, value)
}

// UnsetTestEnv unsets an environment variable for testing
func UnsetTestEnv(key string) error {
	return os.Unsetenv(key)
}

// GetRequiredEnv gets a required environment variable or panics
func GetRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("%s environment variable is required", key))
	}
	return value
}

// GetEnvWithDefault gets an environment variable with a default value
func GetEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetEnvAsList gets an environment variable as a list split by commas
func GetEnvAsList(key string) []string {
	value := os.Getenv(key)
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}
