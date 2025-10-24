package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap/zapcore"
)

// GetLogOutput returns the appropriate log output based on service name
func GetLogOutput(serviceName string) (zapcore.WriteSyncer, error) {
	// Use user's home directory for logs
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %v", err)
	}

	// Create log directory if it doesn't exist
	logDir := filepath.Join(homeDir, ".universal-middleware/log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Open log file
	logFile := filepath.Join(logDir, serviceName+".log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	return zapcore.Lock(zapcore.AddSync(f)), nil
}
