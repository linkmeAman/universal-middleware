package logger

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ctxKey string

const loggerKey ctxKey = "logger"

// Logger wraps zap.Logger with context support
type Logger struct {
	*zap.Logger
}

func New(serviceName string, level string) (*Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Parse level
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		lvl = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(lvl)

	// Add service name
	config.InitialFields = map[string]interface{}{
		"service": serviceName,
	}

	// Get file output
	fileOutput, err := GetLogOutput(serviceName)
	if err != nil {
		return nil, err
	}

	// Create tee for both file and console output
	consoleOutput := zapcore.Lock(os.Stdout)
	config.OutputPaths = []string{"stdout", serviceName + ".log"}

	// Build encoder
	encoder := zapcore.NewJSONEncoder(config.EncoderConfig)

	// Create core
	core := zapcore.NewTee(
		zapcore.NewCore(encoder, consoleOutput, config.Level),
		zapcore.NewCore(encoder, fileOutput, config.Level),
	)

	// Build logger
	baseLogger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return &Logger{Logger: baseLogger}, nil
}

// WithContext adds logger to context
func WithContext(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext retrieves logger from context
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey).(*Logger); ok {
		return logger
	}
	// Return no-op logger if not found
	return &Logger{Logger: zap.NewNop()}
}

// WithFields creates child logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	return &Logger{Logger: l.With(zapFields...)}
}

// WithTraceID adds trace ID to logger
func (l *Logger) WithTraceID(traceID string) *Logger {
	return &Logger{Logger: l.With(zap.String("trace_id", traceID))}
}
