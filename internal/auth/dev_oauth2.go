package auth

import (
	"context"
	"time"

	"golang.org/x/oauth2"
)

// DevOAuth2Provider provides a development-mode OAuth2 provider that accepts any credentials
type DevOAuth2Provider struct{}

// NewDevOAuth2Provider creates a new development OAuth2 provider
func NewDevOAuth2Provider() OAuth2Provider {
	return &DevOAuth2Provider{}
}

// GetAuthURL returns a dummy authorization URL
func (p *DevOAuth2Provider) GetAuthURL(state string) string {
	return "/auth/dev-login?state=" + state
}

// Exchange simulates exchanging a code for a token
func (p *DevOAuth2Provider) Exchange(_ context.Context, _ string) (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken:  "dev-access-token",
		TokenType:    "Bearer",
		RefreshToken: "dev-refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour),
	}, nil
}

// TokenSource returns a token source that always returns a valid token
func (p *DevOAuth2Provider) TokenSource(_ context.Context, _ *oauth2.Token) oauth2.TokenSource {
	return oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken:  "dev-access-token",
		TokenType:    "Bearer",
		RefreshToken: "dev-refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour),
	})
}

// VerifyToken simulates token verification
func (p *DevOAuth2Provider) VerifyToken(_ context.Context, _ string) (*Claims, error) {
	return &Claims{
		Subject:       "dev-user",
		Name:          "Development User",
		Email:         "dev@example.com",
		EmailVerified: true,
		PreferredName: "Dev",
		Role:          "admin",
		Groups:        []string{"developers"},
	}, nil
}

// GetUserInfo returns static user info for development
func (p *DevOAuth2Provider) GetUserInfo(_ context.Context, _ *oauth2.Token) (*Claims, error) {
	return &Claims{
		Subject:       "dev-user",
		Name:          "Development User",
		Email:         "dev@example.com",
		EmailVerified: true,
		PreferredName: "Dev",
		Role:          "admin",
		Groups:        []string{"developers"},
	}, nil
}
