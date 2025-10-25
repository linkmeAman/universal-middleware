package auth

import (
	"context"

	"gopkg.in/square/go-jose.v2/jwt"
)

// OPAAuthorizer defines the interface for authorization providers
type OPAAuthorizer interface {
	IsAllowed(ctx context.Context, input interface{}) (bool, error)
	RefreshPolicies(ctx context.Context) error
}

// Claims represents the JWT claims we care about
type Claims struct {
	Subject          string   `json:"sub"`
	Name             string   `json:"name"`
	Email            string   `json:"email"`
	EmailVerified    bool     `json:"email_verified"`
	PreferredName    string   `json:"preferred_username,omitempty"`
	GivenName        string   `json:"given_name,omitempty"`
	FamilyName       string   `json:"family_name,omitempty"`
	Locale           string   `json:"locale,omitempty"`
	Picture          string   `json:"picture,omitempty"`
	Role             string   `json:"role,omitempty"`
	Groups           []string `json:"groups,omitempty"`
	Scope            []string `json:"scope,omitempty"`
	IdentityProvider string   `json:"idp,omitempty"`
	jwt.Claims
}

// Endpoints represents the OAuth2/OIDC endpoints
type Endpoints struct {
	AuthorizationEndpoint string
	TokenEndpoint         string
	UserInfoEndpoint      string
	JWKSURI               string
}
