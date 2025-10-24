package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/linkmeAman/universal-middleware/internal/auth"
	"github.com/linkmeAman/universal-middleware/test/testutil"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPolicy = `
package authz

default allow = false

allow {
    input.method == "GET"
    input.path == ["api", "public"]
}

allow {
    input.method == "POST"
    input.path == ["api", "users"]
    input.user.role == "admin"
}
`

func setupOPA(t *testing.T) *auth.OPAAuthorizer {
	roleData := `{"roles":{"admin":{"permissions":["create_user","read_user"]},"user":{"permissions":["read_user"]}}}`
	store := inmem.NewFromReader(strings.NewReader(roleData))
	log := testutil.NewTestLogger(t)

func setupOPA(t *testing.T) *auth.OPAAuthorizer {
`

func setupOPA(t *testing.T) *auth.OPAAuthorizer {
	store := inmem.NewFromReader(strings.NewReader(`{"roles":{"admin":{"permissions":["create_user","read_user"]},"user":{"permissions":["read_user"]}}}`))
	log := testutil.NewTestLogger(t)
	opa, err := auth.NewOPAAuthorizer(log, store, []byte(testPolicy))
	require.NoError(t, err)
	return opa
}

func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		user           *auth.User
		expectedStatus int
	}{
		{
			name:   "Public endpoint - allowed",
			method: "GET",
			path:   "/api/public",
			user:   nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:   "Admin endpoint - allowed for admin",
			method: "POST",
			path:   "/api/users",
			user: &auth.User{
				ID:    "1",
				Role:  "admin",
				Email: "admin@example.com",
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "Admin endpoint - denied for regular user",
			method: "POST",
			path:   "/api/users",
			user: &auth.User{
				ID:    "2",
				Role:  "user",
				Email: "user@example.com",
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "Unknown endpoint - denied",
			method: "GET",
			path:   "/api/unknown",
			user:   nil,
			expectedStatus: http.StatusForbidden,
		},
	}

	log := testutil.NewTestLogger(t)
	opa := setupOPA(t)
	authMiddleware := NewAuthMiddleware(log, opa)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.user != nil {
				ctx := context.WithValue(req.Context(), auth.UserContextKey, tt.user)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			nextCalled := false

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := authMiddleware.Authorize(next)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusOK {
				assert.True(t, nextCalled, "Next handler should have been called")
			} else {
				assert.False(t, nextCalled, "Next handler should not have been called")
			}

			if tt.expectedStatus == http.StatusForbidden {
				var response map[string]interface{}
				err := json.NewDecoder(rr.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
				assert.Equal(t, "Forbidden", response["error"])
			}
		})
	}
}

func TestAuthMiddleware_PolicyEvaluation(t *testing.T) {
	log := testutil.NewTestLogger(t)
	opa := setupOPA(t)
	authMiddleware := NewAuthMiddleware(log, opa)

	tests := []struct {
		name        string
		setupReq    func() *http.Request
		assertError func(t *testing.T, err error)
	}{
		{
			name: "Invalid path segments",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "//invalid//path", nil)
				return req
			},
			assertError: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid path segments")
			},
		},
		{
			name: "Invalid user context",
			setupReq: func() *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/api/users", nil)
				ctx := context.WithValue(req.Context(), auth.UserContextKey, "invalid-user")
				return req.WithContext(ctx)
			},
			assertError: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid user context")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			rr := httptest.NewRecorder()

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Error("Next handler should not be called")
			})

			handler := authMiddleware.Authorize(next)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusForbidden, rr.Code)
			var response map[string]interface{}
			err := json.NewDecoder(rr.Body).Decode(&response)
			assert.NoError(t, err)
			assert.Contains(t, response, "error")
		})
	}
}