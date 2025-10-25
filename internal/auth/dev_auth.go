package auth

import (
	"context"

	"github.com/linkmeAman/universal-middleware/pkg/logger"
)

// DevOPAAuthorizer provides a development-mode authorizer that allows all requests
type DevOPAAuthorizer struct {
	log *logger.Logger
}

// NewDevOPAAuthorizer creates a new development OPA authorizer
func NewDevOPAAuthorizer(log *logger.Logger) *DevOPAAuthorizer {
	return &DevOPAAuthorizer{
		log: log,
	}
}

// IsAllowed always returns true in development mode
func (a *DevOPAAuthorizer) IsAllowed(ctx context.Context, input interface{}) (bool, error) {
	return true, nil
}

// RefreshPolicies is a no-op in development mode
func (a *DevOPAAuthorizer) RefreshPolicies(ctx context.Context) error {
	a.log.Info("Skipping policy refresh in development mode")
	return nil
}
