package middleware

import (
	"context"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Key type for context values
type key int

const (
	requestIDKey key = iota
)

var requestCounter uint64

// ResponseWriter wraps http.ResponseWriter to capture the status code
type ResponseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// NewResponseWriter creates a new ResponseWriter
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{ResponseWriter: w, status: http.StatusOK}
}

// WriteHeader captures the status code and writes it
func (rw *ResponseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true
}

// Status returns the HTTP status of the request
func (rw *ResponseWriter) Status() int {
	return rw.status
}

// GenerateRequestID generates a unique request ID
func GenerateRequestID() string {
	return strconv.FormatUint(atomic.AddUint64(&requestCounter, 1), 10)
}

// Chain represents a chain of middleware handlers
type Chain struct {
	middlewares []func(http.Handler) http.Handler
}

// NewChain creates a new middleware chain
func NewChain(log *logger.Logger, m *metrics.Metrics) *Chain {
	return &Chain{
		middlewares: []func(http.Handler) http.Handler{
			WithRequestID,
			WithRecovery(log),
			WithTracing(otel.GetTracerProvider().Tracer("http")),
			WithLogging(log),
			WithMetrics(m),
		},
	}
}

// Use adds middleware to the chain
func (c *Chain) Use(middleware ...func(http.Handler) http.Handler) {
	c.middlewares = append(c.middlewares, middleware...)
}

// Then wraps the final handler with all middleware in the chain
func (c *Chain) Then(h http.Handler) http.Handler {
	if h == nil {
		h = http.DefaultServeMux
	}

	for i := len(c.middlewares) - 1; i >= 0; i-- {
		h = c.middlewares[i](h)
	}

	return h
}

// WithRequestID adds a request ID to the context
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = GenerateRequestID()
		}
		ctx = context.WithValue(ctx, requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WithRecovery recovers from panics and logs them
func WithRecovery(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Error("Request panic recovered",
						zap.Any("error", err),
						zap.String("url", r.URL.String()),
						zap.String("method", r.Method),
					)
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// WithTracing adds OpenTelemetry tracing
func WithTracing(tracer trace.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracer.Start(r.Context(), "http.request",
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
				),
			)
			defer span.End()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithLogging adds request logging
func WithLogging(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := NewResponseWriter(w)
			start := time.Now()

			next.ServeHTTP(ww, r)

			log.Info("Request completed",
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", time.Since(start)),
			)
		})
	}
}

// WithMetrics adds Prometheus metrics
func WithMetrics(m *metrics.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := NewResponseWriter(w)

			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			m.HTTPRequestDuration.WithLabelValues(
				r.Method,
				r.URL.Path,
				strconv.Itoa(ww.Status()),
			).Observe(duration.Seconds())
		})
	}
}

// WithTimeout adds a timeout to the request context
func WithTimeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
