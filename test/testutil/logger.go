package testutil

import (
	"io"
	"os"
	"testing"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

// NewTestLogger creates a logger suitable for testing
func NewTestLogger(t *testing.T) *logger.Logger {
	// Use zaptest to automatically clean up logs
	zapLogger := zaptest.NewLogger(t, zaptest.Level(zap.InfoLevel))
	return &logger.Logger{Logger: zapLogger}
}

// CaptureLogs captures logs during test execution
func CaptureLogs(t *testing.T) (*logger.Logger, *os.File) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(config.EncoderConfig),
		zapcore.AddSync(w),
		zap.InfoLevel,
	)

	log := &logger.Logger{
		Logger: zap.New(core),
	}

	return log, r
}

// ReadLogs reads captured logs
func ReadLogs(r *os.File) string {
	r.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}
