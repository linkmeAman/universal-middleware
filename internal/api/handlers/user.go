package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/linkmeAman/universal-middleware/internal/api/validation"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/linkmeAman/universal-middleware/pkg/metrics"
	"go.uber.org/zap"
)

// UserHandler handles user-related requests
type UserHandler struct {
	log     *logger.Logger
	metrics *metrics.Metrics
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(log *logger.Logger, m *metrics.Metrics) *UserHandler {
	return &UserHandler{
		log:     log,
		metrics: m,
	}
}

// CreateUser handles user creation
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	// Get validated request from context
	val := r.Context().Value(validation.ValidatedKey)
	if val == nil {
		h.log.Error("No validated request found in context")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	req := val.(*validation.CreateUserRequest)

	// Log the request
	h.log.Info("Creating user",
		zap.String("username", req.Username),
		zap.String("email", req.Email),
	)

	// TODO: Add actual user creation logic here

	// Return success response
	resp := map[string]interface{}{
		"message": "User created successfully",
		"user": map[string]interface{}{
			"username": req.Username,
			"email":    req.Email,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// RegisterRoutes registers the user routes
func (h *UserHandler) RegisterRoutes(r chi.Router, v *validation.Validator) {
	r.Route("/users", func(r chi.Router) {
		// POST /api/v1/users - Create user
		r.With(v.ValidateRequest).Post("/", func(w http.ResponseWriter, r *http.Request) {
			// Set validation struct for this endpoint
			ctx := context.WithValue(r.Context(), validation.ValidationKey, validation.CreateUserRequest{})
			h.CreateUser(w, r.WithContext(ctx))
		})
	})
}
