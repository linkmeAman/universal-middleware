package middleware

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
)

// TracingMiddleware adds OpenTelemetry tracing to requests
type TracingMiddleware struct {
	tracer trace.Tracer
	log    *logger.Logger
}

// NewTracingMiddleware creates a new tracing middleware
func NewTracingMiddleware(serviceName string, log *logger.Logger) *TracingMiddleware {
	return &TracingMiddleware{
		tracer: otel.GetTracerProvider().Tracer(serviceName),
		log:    log,
	}
}

// Trace is middleware that adds tracing to requests
func (m *TracingMiddleware) Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract context from headers
		propagator := otel.GetTextMapPropagator()
		ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Start a new span
		spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		ctx, span := m.tracer.Start(ctx, spanName,
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("http.host", r.Host),
				attribute.String("http.user_agent", r.UserAgent()),
			),
		)
		defer span.End()

		// Inject trace context into response headers
		propagator.Inject(ctx, propagation.HeaderCarrier(w.Header()))

		// Create wrapped response writer to capture status code
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		// Update request with traced context
		r = r.WithContext(ctx)

		// Call next handler
		next.ServeHTTP(rw, r)

		// Record response details
		span.SetAttributes(
			attribute.Int("http.status_code", rw.status),
		)

		if rw.status >= 400 {
			span.SetStatus(codes.Error, http.StatusText(rw.status))
		}
	})
}
