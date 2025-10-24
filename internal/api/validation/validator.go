package validation

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.uber.org/zap"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
}

// Validator handles request validation
type Validator struct {
	log *logger.Logger
}

// New creates a new Validator instance
func New(log *logger.Logger) *Validator {
	return &Validator{
		log: log,
	}
}

// ValidateRequest is middleware that validates the request body against a struct
func (v *Validator) ValidateRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip validation for GET, HEAD, OPTIONS
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Get validation struct from context
		validationStruct := r.Context().Value(ValidationKey)
		if validationStruct == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Create a new instance of the validation struct
		val := reflect.New(reflect.TypeOf(validationStruct)).Interface()

		// Decode request body
		if err := json.NewDecoder(r.Body).Decode(val); err != nil {
			v.log.Error("Failed to decode request body", zap.Error(err))
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate the struct
		if err := validate.Struct(val); err != nil {
			validationErrors := err.(validator.ValidationErrors)
			errors := make([]string, 0)
			for _, e := range validationErrors {
				errors = append(errors, formatValidationError(e))
			}

			v.log.Error("Validation failed",
				zap.Strings("errors", errors),
				zap.String("path", r.URL.Path),
			)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errors": errors,
			})
			return
		}

		// Store validated struct back in context
		*r = *r.WithContext(context.WithValue(r.Context(), ValidatedKey, val))

		next.ServeHTTP(w, r)
	})
}

// ValidationKey is used to store the validation struct type in context
type contextKey string

const (
	// ValidationKey is the key for storing validation struct in context
	ValidationKey contextKey = "validation_struct"
	// ValidatedKey is the key for storing validated struct in context
	ValidatedKey contextKey = "validated_struct"
)

// formatValidationError converts a validation error to a human-readable message
func formatValidationError(e validator.FieldError) string {
	field := strings.ToLower(e.Field())
	switch e.Tag() {
	case "required":
		return field + " is required"
	case "email":
		return field + " must be a valid email address"
	case "min":
		return field + " must be at least " + e.Param() + " characters long"
	case "max":
		return field + " must be at most " + e.Param() + " characters long"
	default:
		return field + " is invalid"
	}
}
