package validation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/linkmeAman/universal-middleware/test/testutil"
	"github.com/stretchr/testify/assert"
)

// TestRequest is a test struct for validation
type TestRequest struct {
	Username string `json:"username" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Age      int    `json:"age" validate:"required,gte=18"`
}

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name           string
		reqBody        interface{}
		validationObj  interface{}
		expectedStatus int
	}{
		{
			name: "Valid request",
			reqBody: TestRequest{
				Username: "testuser",
				Email:    "test@example.com",
				Password: "password123",
				Age:      25,
			},
			validationObj:  TestRequest{},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Invalid email",
			reqBody: TestRequest{
				Username: "testuser",
				Email:    "invalid-email",
				Password: "password123",
				Age:      25,
			},
			validationObj:  TestRequest{},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "Missing required fields",
			reqBody: TestRequest{
				Username: "testuser",
			},
			validationObj:  TestRequest{},
			expectedStatus: http.StatusBadRequest,
		},
	}

	log := testutil.NewTestLogger(t)
	validator := New(log)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			body, err := json.Marshal(tt.reqBody)
			assert.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
			req = req.WithContext(context.WithValue(req.Context(), ValidationKey, tt.validationObj))

			rr := httptest.NewRecorder()

			// Create test handler
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			// Test the middleware
			handler := validator.ValidateRequest(next)
			handler.ServeHTTP(rr, req)

			// Verify response
			assert.Equal(t, tt.expectedStatus, rr.Code)

			// Verify if next handler was called for valid requests
			if tt.expectedStatus == http.StatusOK {
				assert.True(t, nextCalled)
			} else {
				assert.False(t, nextCalled)
			}

			// For invalid requests, verify error response
			if tt.expectedStatus != http.StatusOK {
				var response map[string]interface{}
				err := json.NewDecoder(rr.Body).Decode(&response)
				assert.NoError(t, err)
				assert.Contains(t, response, "errors")
				assert.NotEmpty(t, response["errors"])
			}
		})
	}
}

func TestValidateRequest_SkipMethods(t *testing.T) {
	skipMethods := []string{http.MethodGet, http.MethodHead, http.MethodOptions}
	log := testutil.NewTestLogger(t)
	validator := New(log)

	for _, method := range skipMethods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/test", nil)
			rr := httptest.NewRecorder()

			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := validator.ValidateRequest(next)
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.True(t, nextCalled, "Next handler should be called for %s method", method)
		})
	}
}
