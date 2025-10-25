package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
	"go.uber.org/zap"
)

// Middleware wraps common dependencies for all middleware
type Middleware struct {
	log     *logger.Logger
	logger  *zap.Logger
	metrics *metrics.Metrics
}

// New creates a new Middleware instance
func New(log *logger.Logger, m *metrics.Metrics) *Middleware {
	zapLogger, _ := zap.NewProduction()
	return &Middleware{
		log:     log,
		logger:  zapLogger,
		metrics: m,
	}
}

// RequestLogger logs incoming HTTP requests
func (m *Middleware) RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response wrapper to capture status code
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Process request
		next.ServeHTTP(ww, r)

		// Log request details
		m.log.Info("HTTP Request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.Int("status", ww.status),
			zap.Duration("duration", time.Since(start)),
		)
	})
}

// RequestTracker adds request tracking headers
func (m *Middleware) RequestTracker(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add request tracking headers
		w.Header().Set("X-Request-ID", r.Header.Get("X-Request-ID"))
		w.Header().Set("X-Trace-ID", r.Header.Get("X-Trace-ID"))

		next.ServeHTTP(w, r)
	})
}

// MetricsCollector collects request metrics
func (m *Middleware) MetricsCollector(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// (optional) a cheap way to record request bytes:
		if r.ContentLength > 0 {
			ww.bytesRead = r.ContentLength
		}

		next.ServeHTTP(ww, r)

		m.metrics.ObserveHTTP(
			r.Method,
			r.URL.Path,
			strconv.Itoa(ww.Status()), // <- fix: convert int -> string
			time.Since(start),
			ww.BytesWritten(),
			ww.BytesRead(),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code and response size
type responseWriter struct {
	http.ResponseWriter
	status     int
	bytesRead  int64
	bytesWrote int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWrote += int64(n)
	return n, err
}

func (rw *responseWriter) Status() int {
	return rw.status
}

func (rw *responseWriter) BytesWritten() int {
	return int(rw.bytesWrote)
}

func (rw *responseWriter) BytesRead() int {
	return int(rw.bytesRead)
}
