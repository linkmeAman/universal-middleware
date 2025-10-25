package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/linkmeAman/universal-middleware/internal/ratelimit"
)

// RateLimit creates a middleware that applies rate limiting
func (m *Middleware) RateLimit(limiter *ratelimit.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get rate limit key from request
			key := getRateLimitKey(r)

			// Check rate limit
			allowed, err := limiter.Allow(r.Context(), key)
			if err != nil {
				m.logger.Error("Rate limit check failed",
					zap.String("key", key),
					zap.Error(err))
				// On error, we'll allow the request but log the error
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				// Get remaining tokens for the response header
				remaining, _ := limiter.GetRemainingTokens(r.Context(), key)

				// Set rate limit headers
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limiter.MaxTokens()))
				w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", limiter.NextReset()))

				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			// Set rate limit headers on success too
			remaining, _ := limiter.GetRemainingTokens(r.Context(), key)
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limiter.MaxTokens()))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", limiter.NextReset()))

			next.ServeHTTP(w, r)
		})
	}
}

// getRateLimitKey generates a key for rate limiting based on the request
func getRateLimitKey(r *http.Request) string {
	// Start with the remote IP
	key := r.RemoteAddr

	// Add API key if present
	if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
		key = apiKey
	}

	// Add user ID if authenticated
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		key = userID
	}

	// Add path to make the limit per-endpoint
	key = fmt.Sprintf("%s:%s", key, r.URL.Path)

	// Clean the key
	key = strings.Replace(key, " ", "_", -1)
	key = strings.Replace(key, ":", "_", -1)

	return fmt.Sprintf("ratelimit:%s", key)
}
