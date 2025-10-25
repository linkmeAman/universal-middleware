package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"go.uber.org/zap"
)

var validate *validator.Validate

func init() {
	validate = validator.New()

	// Register custom username validation
	validate.RegisterValidation("username", validateUsername)
	validate.RegisterValidation("password", validatePassword)
}

// validateUsername checks if username follows required format
func validateUsername(fl validator.FieldLevel) bool {
	username := fl.Field().String()
	// Allow only alphanumeric and underscore
	// Must start with a letter
	// Length 3-50 characters
	usernameRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{2,49}$`)
	return usernameRegex.MatchString(username)
}

// validatePassword checks if password is strong enough
func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	// At least 8 characters
	// At least 1 uppercase letter
	// At least 1 lowercase letter
	// At least 1 number
	// At least 1 special character
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasSpecial := regexp.MustCompile(`[!@#$%^&*]`).MatchString(password)

	return len(password) >= 8 && hasUpper && hasLower && hasNumber && hasSpecial
}

// ValidationKey is the context key for validation struct
type validationContextKey string

// ValidationKey is the key for storing validation struct in context
const ValidationKey validationContextKey = "validation"

// ValidatedKey is the key for storing validated struct in context
const ValidatedKey validationContextKey = "validated"

// Validator handles request validation
type Validator struct {
	log      *logger.Logger
	validate *validator.Validate
}

// NewValidator creates a new validator instance
func NewValidator(log *logger.Logger) *Validator {
	return &Validator{
		log:      log,
		validate: validate,
	}
}

// ValidateRequest validates incoming requests based on the validation struct in context
func (v *Validator) ValidateRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip validation for GET, HEAD, OPTIONS
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Get validation struct type from context
		valType, ok := r.Context().Value(ValidationKey).(interface{})
		if !ok {
			v.log.Error("No validation type specified")
			http.Error(w, "No validation type specified", http.StatusInternalServerError)
			return
		}

		// Create a new instance of the validation struct
		val := reflect.New(reflect.TypeOf(valType)).Interface()

		// Parse request body into validation struct
		if err := json.NewDecoder(r.Body).Decode(val); err != nil {
			v.log.Error("Failed to decode request body",
				zap.Error(err),
				zap.String("path", r.URL.Path),
			)
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Validate the struct
		if err := v.validate.Struct(val); err != nil {
			validationErrors := []string{}
			for _, err := range err.(validator.ValidationErrors) {
				// Convert validation error to readable message
				msg := fmt.Sprintf("Field '%s' failed validation: %s",
					toSnakeCase(err.Field()),
					getValidationErrorMsg(err))
				validationErrors = append(validationErrors, msg)
			}

			v.log.Error("Validation failed",
				zap.Strings("errors", validationErrors),
				zap.String("path", r.URL.Path),
			)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "Validation failed",
				"details": validationErrors,
			})
			return
		}

		// Store validated request in context
		ctx := context.WithValue(r.Context(), ValidatedKey, val)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Helper function to convert CamelCase to snake_case
func toSnakeCase(str string) string {
	var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

// Helper function to get readable validation error messages
func getValidationErrorMsg(err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "min":
		return fmt.Sprintf("must be at least %s characters long", err.Param())
	case "max":
		return fmt.Sprintf("must not be longer than %s characters", err.Param())
	case "alphanum":
		return "must contain only alphanumeric characters"
	case "containsany":
		return fmt.Sprintf("must contain at least one of these characters: %s", err.Param())
	case "username":
		return "must start with a letter and contain only letters, numbers, and underscores"
	case "password":
		return "must contain at least 8 characters including uppercase, lowercase, number, and special character"
	case "nefield":
		return fmt.Sprintf("must be different from %s", err.Param())
	default:
		return fmt.Sprintf("failed %s validation", err.Tag())
	}
}
