package middleware

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"go.uber.org/zap"

	"github.com/linkmeAman/universal-middleware/internal/loadbalancer"
)

// LoadBalancer creates a middleware that distributes requests across multiple backends
func (m *Middleware) LoadBalancer(lb *loadbalancer.LoadBalancer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if we should handle this request or pass it through
			if shouldLoadBalance(r) {
				backend := lb.NextBackend()
				if backend == nil {
					http.Error(w, "No available backends", http.StatusServiceUnavailable)
					return
				}

				// Parse the backend URL
				target, err := url.Parse(backend.URL)
				if err != nil {
					m.logger.Error("Failed to parse backend URL",
						zap.String("url", backend.URL),
						zap.Error(err))
					http.Error(w, "Internal server error", http.StatusInternalServerError)
					return
				}

				// Create a reverse proxy
				proxy := httputil.NewSingleHostReverseProxy(target)

				// Update the request URL
				r.URL.Host = target.Host
				r.URL.Scheme = target.Scheme
				r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
				r.Host = target.Host

				// Log the proxied request
				m.logger.Debug("Proxying request",
					zap.String("path", r.URL.Path),
					zap.String("backend", backend.URL))

				// Proxy the request
				proxy.ServeHTTP(w, r)
				return
			}

			// Pass through to next handler if we shouldn't load balance
			next.ServeHTTP(w, r)
		})
	}
}

// shouldLoadBalance determines if a request should be load balanced
func shouldLoadBalance(r *http.Request) bool {
	// Add logic here to determine which requests should be load balanced
	// For example, based on path prefix, headers, etc.
	return r.URL.Path != "/health" && r.URL.Path != "/metrics"
}
