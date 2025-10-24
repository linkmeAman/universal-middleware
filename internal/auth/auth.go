package auth

import (
	"context"

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

// OPAAuthorizer handles authorization using OPA
type OPAAuthorizer struct {
	store storage.Store
	query rego.PreparedEvalQuery
	log   *logger.Logger
}

// AuthConfig represents the configuration for OPA authorization
type AuthConfig struct {
	PolicyFile string
	Store      storage.Store
}

// NewOPAAuthorizer creates a new OPA authorizer instance
func NewOPAAuthorizer(log *logger.Logger, store storage.Store, policy []byte) (*OPAAuthorizer, error) {
	ctx := context.Background()

	// Prepare Rego query
	query, err := rego.New(
		rego.Query("data.authz.allow"),
		rego.Module("authz.rego", string(policy)),
		rego.Store(store),
	).PrepareForEval(ctx)

	if err != nil {
		return nil, err
	}

	return &OPAAuthorizer{
		store: store,
		query: query,
		log:   log,
	}, nil
}

// IsAllowed evaluates if the request is allowed based on the policy
func (a *OPAAuthorizer) IsAllowed(ctx context.Context, input interface{}) (bool, error) {
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
