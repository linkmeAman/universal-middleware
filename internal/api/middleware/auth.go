package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/linkmeAman/universal-middleware/internal/auth"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// AuthMiddleware handles request authorization using OPA policies and OAuth2
type AuthMiddleware struct {
	log        *logger.Logger
	authorizer auth.OPAAuthorizer
	oauth2     auth.OAuth2Provider
	store      sessions.Store
}

// NewAuthMiddleware creates a new AuthMiddleware instance
func NewAuthMiddleware(log *logger.Logger, authorizer auth.OPAAuthorizer, provider auth.OAuth2Provider, store sessions.Store) *AuthMiddleware {
	return &AuthMiddleware{
		log:        log,
		authorizer: authorizer,
		oauth2:     provider,
		store:      store,
	}
}

// Authorize middleware checks if the request is allowed based on OPA policies
func (m *AuthMiddleware) Authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for login and callback routes
		if strings.HasPrefix(r.URL.Path, "/auth/login") || strings.HasPrefix(r.URL.Path, "/auth/callback") {
			next.ServeHTTP(w, r)
			return
		}
		// Parse path into segments
		path := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(path) == 0 {
			m.log.Error("Invalid path segments", zap.String("path", r.URL.Path))
			m.sendError(w, "Invalid request path")
			return
		}

		// Get user from session
		session, err := m.store.Get(r, "auth-session")
		if err != nil {
			m.log.Error("Failed to get session", zap.Error(err))
			m.sendError(w, "Session error")
			return
		}

		var user *auth.User
		if session.Values["user_id"] != nil {
			user = auth.GetUserFromSession(session)
			if user == nil {
				m.log.Error("Invalid user session data")
				m.sendError(w, "Invalid session")
				return
			}

			// Check if access token needs refresh
			expiry, ok := session.Values["token_expiry"].(time.Time)
			if ok && time.Until(expiry) < 5*time.Minute {
				refreshToken, ok := session.Values["refresh_token"].(string)
				if ok {
					newToken, err := m.oauth2.TokenSource(r.Context(), &oauth2.Token{
						RefreshToken: refreshToken,
					}).Token()
					if err == nil {
						session.Values["access_token"] = newToken.AccessToken
						session.Values["refresh_token"] = newToken.RefreshToken
						session.Values["token_expiry"] = newToken.Expiry
						session.Save(r, w)
					} else {
						m.log.Error("Failed to refresh token", zap.Error(err))
					}
				}
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
