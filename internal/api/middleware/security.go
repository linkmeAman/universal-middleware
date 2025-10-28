package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Security middleware with rate limiting and authentication
type SecurityMiddleware struct {
	jwtSecret   []byte
	redisClient *redis.Client
	log         *zap.Logger

	// Rate limiting per IP
	limiters map[string]*rate.Limiter
}

// NewSecurityMiddleware creates security middleware
func NewSecurityMiddleware(jwtSecret string, redisAddr string, log *zap.Logger) *SecurityMiddleware {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   2, // Separate DB for rate limiting
	})

	return &SecurityMiddleware{
		jwtSecret:   []byte(jwtSecret),
		redisClient: rdb,
		log:         log,
		limiters:    make(map[string]*rate.Limiter),
	}
}

// AuthenticateWebSocket validates JWT token for WebSocket upgrade
func (s *SecurityMiddleware) AuthenticateWebSocket(r *http.Request) (string, error) {
	// Extract token from query parameter or header
	token := r.URL.Query().Get("token")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if token == "" {
		return "", errors.New("missing authentication token")
	}

	// Validate JWT
	claims, err := s.validateJWT(token)
	if err != nil {
		s.log.Warn("Invalid WebSocket authentication",
			zap.Error(err),
			zap.String("remote_addr", r.RemoteAddr))
		return "", err
	}

	// Extract user ID from claims
	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", errors.New("invalid user_id in token")
	}

	return userID, nil
}

// RateLimitMiddleware implements distributed rate limiting
func (s *SecurityMiddleware) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get client identifier (IP or user ID)
		clientID := s.getClientIdentifier(r)

		// Check rate limit in Redis (sliding window)
		allowed, remaining, resetTime, err := s.checkRateLimit(r.Context(), clientID)
		if err != nil {
			s.log.Error("Rate limit check failed", zap.Error(err))
			// Fail open - allow request but log error
			next.ServeHTTP(w, r)
			return
		}

		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", "100")
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime))

		if !allowed {
			s.log.Warn("Rate limit exceeded",
				zap.String("client_id", clientID),
				zap.String("path", r.URL.Path))

			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// checkRateLimit implements sliding window rate limiting in Redis
func (s *SecurityMiddleware) checkRateLimit(ctx context.Context, clientID string) (bool, int, int64, error) {
	key := fmt.Sprintf("ratelimit:%s", clientID)
	now := time.Now().Unix()
	window := int64(60) // 60 second window
	limit := 100        // 100 requests per window

	// Lua script for atomic sliding window rate limiting
	script := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		
		-- Remove old entries
		redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
		
		-- Count current requests
		local count = redis.call('ZCARD', key)
		
		if count < limit then
			-- Add current request
			redis.call('ZADD', key, now, now .. '-' .. math.random())
			redis.call('EXPIRE', key, window)
			return {1, limit - count - 1, now + window}
		else
			-- Get oldest entry for reset time
			local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
			local resetTime = tonumber(oldest[2]) + window
			return {0, 0, resetTime}
		end
	`

	result, err := s.redisClient.Eval(ctx, script, []string{key}, now, window, limit).Result()
	if err != nil {
		return false, 0, 0, err
	}

	res := result.([]interface{})
	allowed := res[0].(int64) == 1
	remaining := int(res[1].(int64))
	resetTime := res[2].(int64)

	return allowed, remaining, resetTime, nil
}

// validateJWT validates a JWT token
func (s *SecurityMiddleware) validateJWT(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Check expiration
		if exp, ok := claims["exp"].(float64); ok {
			if time.Unix(int64(exp), 0).Before(time.Now()) {
				return nil, errors.New("token expired")
			}
		}
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// getClientIdentifier extracts client identifier for rate limiting
func (s *SecurityMiddleware) getClientIdentifier(r *http.Request) string {
	// Try to get user ID from JWT first
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if claims, err := s.validateJWT(token); err == nil {
			if userID, ok := claims["user_id"].(string); ok {
				return fmt.Sprintf("user:%s", userID)
			}
		}
	}

	// Fall back to IP address
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}

	// Hash IP for privacy
	hash := sha256.Sum256([]byte(ip))
	return fmt.Sprintf("ip:%s", hex.EncodeToString(hash[:8]))
}

// ErrorHandler provides comprehensive error handling
type ErrorHandler struct {
	log *zap.Logger
}

// NewErrorHandler creates an error handler
func NewErrorHandler(log *zap.Logger) *ErrorHandler {
	return &ErrorHandler{log: log}
}

// AppError represents an application error with context
type AppError struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	StatusCode int                    `json:"-"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Internal   error                  `json:"-"`
}

// Error implements error interface
func (e *AppError) Error() string {
	return e.Message
}

// Common error constructors
func NewNotFoundError(resource string, id string) *AppError {
	return &AppError{
		Code:       "NOT_FOUND",
		Message:    fmt.Sprintf("%s not found", resource),
		StatusCode: http.StatusNotFound,
		Details: map[string]interface{}{
			"resource": resource,
			"id":       id,
		},
	}
}

func NewValidationError(field string, issue string) *AppError {
	return &AppError{
		Code:       "VALIDATION_ERROR",
		Message:    "Invalid request",
		StatusCode: http.StatusBadRequest,
		Details: map[string]interface{}{
			"field": field,
			"issue": issue,
		},
	}
}

func NewRateLimitError() *AppError {
	return &AppError{
		Code:       "RATE_LIMIT_EXCEEDED",
		Message:    "Too many requests",
		StatusCode: http.StatusTooManyRequests,
	}
}

func NewServiceUnavailableError(service string, err error) *AppError {
	return &AppError{
		Code:       "SERVICE_UNAVAILABLE",
		Message:    "Service temporarily unavailable",
		StatusCode: http.StatusServiceUnavailable,
		Details: map[string]interface{}{
			"service": service,
		},
		Internal: err,
	}
}

// HandleError processes errors and sends appropriate response
func (h *ErrorHandler) HandleError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *AppError

	// Convert to AppError if not already
	if !errors.As(err, &appErr) {
		// Unknown error - log and return generic message
		h.log.Error("Unhandled error",
			zap.Error(err),
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method))

		appErr = &AppError{
			Code:       "INTERNAL_ERROR",
			Message:    "An internal error occurred",
			StatusCode: http.StatusInternalServerError,
			Internal:   err,
		}
	} else if appErr.Internal != nil {
		// Log internal error for debugging
		h.log.Error("Application error",
			zap.Error(appErr.Internal),
			zap.String("code", appErr.Code),
			zap.String("path", r.URL.Path))
	}

	// Add request ID to error details
	if appErr.Details == nil {
		appErr.Details = make(map[string]interface{})
	}
	if requestID := r.Context().Value("request_id"); requestID != nil {
		appErr.Details["request_id"] = requestID
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.StatusCode)

	// Don't expose internal error details to client
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    appErr.Code,
			"message": appErr.Message,
			"details": appErr.Details,
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode error response", zap.Error(err))
	}
}

// CircuitBreaker implements circuit breaker pattern for external calls
type CircuitBreaker struct {
	maxFailures int
	resetTime   time.Duration

	failures    int
	lastFailure time.Time
	state       string // "closed", "open", "half-open"
	mu          sync.RWMutex
}

// NewCircuitBreaker creates a circuit breaker
func NewCircuitBreaker(maxFailures int, resetTime time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
		resetTime:   resetTime,
		state:       "closed",
	}
}

// Call executes function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()

	// Check if circuit should transition from open to half-open
	if cb.state == "open" {
		if time.Since(cb.lastFailure) > cb.resetTime {
			cb.state = "half-open"
			cb.failures = 0
		} else {
			cb.mu.Unlock()
			return NewServiceUnavailableError("circuit_breaker", errors.New("circuit open"))
		}
	}

	cb.mu.Unlock()

	// Execute function
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.failures >= cb.maxFailures {
			cb.state = "open"
		}

		return err
	}

	// Success - reset circuit
	if cb.state == "half-open" {
		cb.state = "closed"
	}
	cb.failures = 0

	return nil
}

// State returns current circuit breaker state
func (cb *CircuitBreaker) State() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// HealthChecker monitors service health
type HealthChecker struct {
	checks map[string]func(context.Context) error
	mu     sync.RWMutex
}

// NewHealthChecker creates a health checker
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]func(context.Context) error),
	}
}

// RegisterCheck adds a health check
func (h *HealthChecker) RegisterCheck(name string, fn func(context.Context) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = fn
}

// CheckHealth runs all health checks
func (h *HealthChecker) CheckHealth(ctx context.Context) map[string]string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make(map[string]string)

	for name, check := range h.checks {
		if err := check(ctx); err != nil {
			results[name] = fmt.Sprintf("unhealthy: %v", err)
		} else {
			results[name] = "healthy"
		}
	}

	return results
}
