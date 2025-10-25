package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/sessions"
	"github.com/linkmeAman/universal-middleware/internal/auth"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	log      *logger.Logger
	metrics  *metrics.Metrics
	provider auth.OAuth2Provider
	store    sessions.Store
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(log *logger.Logger, m *metrics.Metrics, provider auth.OAuth2Provider, store sessions.Store) *AuthHandler {
	return &AuthHandler{
		log:      log,
		metrics:  m,
		provider: provider,
		store:    store,
	}
}

// Login initiates the OAuth2/OIDC login flow
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.Get(r, "auth-session")
	if err != nil {
		h.log.Error("Failed to get session", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	state := generateRandomState()
	session.Values["oauth_state"] = state
	if err := session.Save(r, w); err != nil {
		h.log.Error("Failed to save session", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	authURL := h.provider.GetAuthURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// Callback handles the OAuth2/OIDC callback
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.Get(r, "auth-session")
	if err != nil {
		h.log.Error("Failed to get session", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Verify state
	expectedState, ok := session.Values["oauth_state"].(string)
	if !ok || r.URL.Query().Get("state") != expectedState {
		h.log.Error("Invalid OAuth state")
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	// Exchange code for token
	code := r.URL.Query().Get("code")
	token, err := h.provider.Exchange(r.Context(), code)
	if err != nil {
		h.log.Error("Failed to exchange code for token", zap.Error(err))
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}

	// Get user info
	claims, err := h.provider.GetUserInfo(r.Context(), token)
	if err != nil {
		h.log.Error("Failed to get user info", zap.Error(err))
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	// Store user info in session
	session.Values["user_id"] = claims.Subject
	session.Values["user_email"] = claims.Email
	session.Values["user_name"] = claims.Name
	session.Values["access_token"] = token.AccessToken
	session.Values["refresh_token"] = token.RefreshToken
	session.Values["token_expiry"] = token.Expiry

	if err := session.Save(r, w); err != nil {
		h.log.Error("Failed to save session", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Redirect to dashboard or home page
	http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
}

// Logout ends the user session
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.Get(r, "auth-session")
	if err != nil {
		h.log.Error("Failed to get session", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Clear session
	session.Options.MaxAge = -1
	if err := session.Save(r, w); err != nil {
		h.log.Error("Failed to save session", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// UserInfo returns information about the currently logged in user
func (h *AuthHandler) UserInfo(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.Get(r, "auth-session")
	if err != nil {
		h.log.Error("Failed to get session", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userId, ok := session.Values["user_id"].(string)
	if !ok {
		http.Error(w, "Not logged in", http.StatusUnauthorized)
		return
	}

	userInfo := map[string]interface{}{
		"id":    userId,
		"email": session.Values["user_email"],
		"name":  session.Values["user_name"],
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInfo)
}

// RefreshToken refreshes the OAuth2 token if it's about to expire
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	session, err := h.store.Get(r, "auth-session")
	if err != nil {
		h.log.Error("Failed to get session", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if we need to refresh the token
	expiry, ok := session.Values["token_expiry"].(time.Time)
	if !ok || time.Until(expiry) > 5*time.Minute {
		w.WriteHeader(http.StatusOK)
		return
	}

	refreshToken, ok := session.Values["refresh_token"].(string)
	if !ok {
		h.log.Error("No refresh token found in session")
		http.Error(w, "No refresh token", http.StatusUnauthorized)
		return
	}

	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	// Refresh the token
	newToken, err := h.provider.TokenSource(r.Context(), token).Token()
	if err != nil {
		h.log.Error("Failed to refresh token", zap.Error(err))
		http.Error(w, "Failed to refresh token", http.StatusInternalServerError)
		return
	}

	// Update session with new token
	session.Values["access_token"] = newToken.AccessToken
	session.Values["refresh_token"] = newToken.RefreshToken
	session.Values["token_expiry"] = newToken.Expiry

	if err := session.Save(r, w); err != nil {
		h.log.Error("Failed to save session", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// RegisterRoutes registers the authentication routes
func (h *AuthHandler) RegisterRoutes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", h.Login)
		r.Get("/callback", h.Callback)
		r.Get("/logout", h.Logout)
		r.Get("/userinfo", h.UserInfo)
		r.Post("/refresh", h.RefreshToken)
	})
}

// Helper function to generate a random state string
func generateRandomState() string {
	// Generate 32 random bytes
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	// Encode as base64url without padding
	state := base64.RawURLEncoding.EncodeToString(b)
	return state
}
