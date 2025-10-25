package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"go.uber.org/zap"
)

// User represents an authenticated user
type User struct {
	ID    string `json:"id"`
	Role  string `json:"role"`
	Email string `json:"email"`
}

// UserContextKey is used to store user information in context
type contextKey string

const UserContextKey contextKey = "user"

// opaAuthorizer implements the OPAAuthorizer interface
type opaAuthorizer struct {
	store      storage.Store
	query      rego.PreparedEvalQuery
	log        *logger.Logger
	endpoint   string
	policyPath string
	policy     []byte
}

// AuthConfig represents the configuration for OPA authorization
type AuthConfig struct {
	PolicyFile string
	Store      storage.Store
}

// NewOPAAuthorizer creates a new OPA authorizer instance
func NewOPAAuthorizer(endpoint, policyPath string, log *logger.Logger) (OPAAuthorizer, error) {
	ctx := context.Background()

	// Prepare Rego query
	options := []func(*rego.Rego){
		rego.Query(fmt.Sprintf("data.%s.allow", policyPath)),
	}

	if endpoint != "" {
		// If endpoint is provided, load policies from HTTP server
		options = append(options, rego.Load([]string{endpoint}, nil))
	}

	query, err := rego.New(options...).PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare OPA query: %v", err)
	}

	return &opaAuthorizer{
		query:      query,
		log:        log,
		endpoint:   endpoint,
		policyPath: policyPath,
	}, nil
}

// IsAllowed evaluates if the request is allowed based on the policy
func (a *opaAuthorizer) IsAllowed(ctx context.Context, input interface{}) (bool, error) {
	results, err := a.query.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		a.log.Error("Policy evaluation failed", zap.Error(err))
		return false, err
	}

	if len(results) == 0 {
		return false, nil
	}

	allowed, ok := results[0].Expressions[0].Value.(bool)
	if !ok {
		return false, nil
	}

	return allowed, nil
}

// RefreshPolicies reloads the OPA policies
func (a *opaAuthorizer) RefreshPolicies(ctx context.Context) error {
	// Implementation from refresh.go
	if a.endpoint == "" {
		return fmt.Errorf("no OPA endpoint configured")
	}

	// Construct policy URL
	policyURL := a.endpoint
	if a.policyPath != "" {
		policyURL = path.Join(policyURL, "v1/policies", a.policyPath)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, policyURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch policies: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch policies: status %d: %s", resp.StatusCode, body)
	}

	// Parse policy response
	var result struct {
		Result struct {
			Policy string `json:"raw"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse policy response: %w", err)
	}

	// Update stored policy
	a.policy = []byte(result.Result.Policy)

	return nil
}
