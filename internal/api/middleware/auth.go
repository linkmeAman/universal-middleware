package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/linkmeAman/universal-middleware/internal/auth"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.uber.org/zap"
)

// AuthMiddleware handles request authorization using OPA policies
type AuthMiddleware struct {
	log        *logger.Logger
	authorizer *auth.OPAAuthorizer
}

// NewAuthMiddleware creates a new AuthMiddleware instance
func NewAuthMiddleware(log *logger.Logger, authorizer *auth.OPAAuthorizer) *AuthMiddleware {
	return &AuthMiddleware{
		log:        log,
		authorizer: authorizer,
	}
}

// Authorize middleware checks if the request is allowed based on OPA policies
func (m *AuthMiddleware) Authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse path into segments
		path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(path) == 0 {
			m.log.Error("Invalid path segments", zap.String("path", r.URL.Path))
			m.sendError(w, "Invalid request path")
			return
		}

		// Get user from context if available
		var user *auth.User
		if u := r.Context().Value(auth.UserContextKey); u != nil {
			var ok bool
			user, ok = u.(*auth.User)
			if !ok {
				m.log.Error("Invalid user context type")
				m.sendError(w, "Invalid user context")
				return
			}
		}

		// Create input for policy evaluation
		input := map[string]interface{}{
			"method": r.Method,
			"path":   path,
		}

		if user != nil {
			input["user"] = map[string]interface{}{
				"id":    user.ID,
				"role":  user.Role,
				"email": user.Email,
			}
		}

		// Evaluate policy
		allowed, err := m.authorizer.IsAllowed(r.Context(), input)
		if err != nil {
			m.log.Error("Policy evaluation failed",
				zap.Error(err),
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
			)
			m.sendError(w, "Authorization failed")
			return
		}

		if !allowed {
			m.log.Warn("Access denied",
				zap.String("path", r.URL.Path),
				zap.String("method", r.Method),
				zap.Any("user", user),
			)
			m.sendError(w, "Forbidden")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (m *AuthMiddleware) sendError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": message,
	})
}
