package auth

import (
	"encoding/base64"
	"encoding/gob"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

func init() {
	// Register types that will be stored in sessions
	gob.Register(&oauth2.Token{})
	gob.Register(time.Time{})
}

// SessionConfig holds session configuration
type SessionConfig struct {
	Secret   string
	Secure   bool
	MaxAge   int
	HTTPOnly bool
	SameSite http.SameSite
	Domain   string
	Path     string
}

// NewSessionStore creates a new session store with the given configuration
func NewSessionStore(config SessionConfig) (sessions.Store, error) {
	// Decode base64 secret
	secret, err := base64.StdEncoding.DecodeString(config.Secret)
	if err != nil {
		return nil, err
	}

	store := sessions.NewCookieStore(secret)
	store.Options = &sessions.Options{
		Path:     config.Path,
		Domain:   config.Domain,
		MaxAge:   config.MaxAge,
		Secure:   config.Secure,
		HttpOnly: config.HTTPOnly,
		SameSite: config.SameSite,
	}

	return store, nil
}

// GetUserFromSession retrieves user information from the session
func GetUserFromSession(s *sessions.Session) *User {
	userId, ok := s.Values["user_id"].(string)
	if !ok {
		return nil
	}

	email, ok := s.Values["user_email"].(string)
	if !ok {
		return nil
	}

	role, ok := s.Values["user_role"].(string)
	if !ok {
		role = "user" // default role
	}

	return &User{
		ID:    userId,
		Email: email,
		Role:  role,
	}
}

// UpdateUserSession updates the session with user information
func UpdateUserSession(s *sessions.Session, user *User) {
	s.Values["user_id"] = user.ID
	s.Values["user_email"] = user.Email
	s.Values["user_role"] = user.Role
}
