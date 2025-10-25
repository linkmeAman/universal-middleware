package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"gopkg.in/square/go-jose.v2/jwt"
)

// OAuth2Config holds the configuration for OAuth2 provider
type OAuth2Config struct {
	ProviderURL  string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// OAuth2Provider represents an OAuth2/OIDC provider interface
type OAuth2Provider interface {
	GetAuthURL(state string) string
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
	TokenSource(ctx context.Context, token *oauth2.Token) oauth2.TokenSource
	VerifyToken(ctx context.Context, token string) (*Claims, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token) (*Claims, error)
}

// oauth2Provider implements OAuth2Provider interface
type oauth2Provider struct {
	config     *oauth2.Config
	httpClient *http.Client
}

// NewOAuth2Provider creates a new OAuth2 provider
func NewOAuth2Provider(cfg OAuth2Config) (OAuth2Provider, error) {
	endpoint, err := discoverEndpoints(context.Background(), cfg.ProviderURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover endpoints: %v", err)
	}

	config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  endpoint.AuthorizationEndpoint,
			TokenURL: endpoint.TokenEndpoint,
		},
		Scopes: cfg.Scopes,
	}

	return &oauth2Provider{
		config:     config,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// GetAuthURL returns the authorization URL with the given state
func (p *oauth2Provider) GetAuthURL(state string) string {
	return p.config.AuthCodeURL(state)
}

// Exchange exchanges the authorization code for tokens
func (p *oauth2Provider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.config.Exchange(ctx, code)
}

// TokenSource returns a token source that refreshes the token as needed
func (p *oauth2Provider) TokenSource(ctx context.Context, token *oauth2.Token) oauth2.TokenSource {
	return p.config.TokenSource(ctx, token)
}

// VerifyToken verifies the JWT token and returns the claims
func (p *oauth2Provider) VerifyToken(ctx context.Context, token string) (*Claims, error) {
	parsed, err := jwt.ParseSigned(token)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	claims := &Claims{}
	if err := parsed.UnsafeClaimsWithoutVerification(claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %v", err)
	}

	// TODO: Verify token signature using JWKS
	// For now, we're just checking expiry

	if claims.Expiry.Time().Before(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// GetUserInfo fetches user information from the provider
func (p *oauth2Provider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*Claims, error) {
	endpoint, err := discoverEndpoints(ctx, p.config.Endpoint.AuthURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", endpoint.UserInfoEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user info: %s", resp.Status)
	}

	var claims Claims
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %v", err)
	}

	return &claims, nil
}

// Helper function to discover OAuth2/OIDC endpoints
func discoverEndpoints(ctx context.Context, issuerURL string) (*Endpoints, error) {
	wellKnown := fmt.Sprintf("%s/.well-known/openid-configuration", issuerURL)
	req, err := http.NewRequestWithContext(ctx, "GET", wellKnown, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery failed: %s", resp.Status)
	}

	var discovery struct {
		AuthorizationEndpoint string `json:"authorization_endpoint"`
		TokenEndpoint         string `json:"token_endpoint"`
		UserInfoEndpoint      string `json:"userinfo_endpoint"`
		JWKSURI               string `json:"jwks_uri"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return nil, err
	}

	return &Endpoints{
		AuthorizationEndpoint: discovery.AuthorizationEndpoint,
		TokenEndpoint:         discovery.TokenEndpoint,
		UserInfoEndpoint:      discovery.UserInfoEndpoint,
		JWKSURI:               discovery.JWKSURI,
	}, nil
}
