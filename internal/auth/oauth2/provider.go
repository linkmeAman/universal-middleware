package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"gopkg.in/square/go-jose.v2/jwt"
)

// Provider represents an OAuth2/OIDC provider
type Provider struct {
	oauth2Config *oauth2.Config
	issuer       string
	endpoints    Endpoints
	httpClient   *http.Client
}

// Endpoints holds the OAuth2/OIDC endpoints
type Endpoints struct {
	Authorization string
	Token         string
	UserInfo      string
	JWKS          string
}

// Config holds the OAuth2/OIDC configuration
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Issuer       string
	Scopes       []string
}

// Claims represents the JWT claims we care about
type Claims struct {
	Subject  string `json:"sub"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Verified bool   `json:"email_verified"`
	jwt.Claims
}

// New creates a new OAuth2/OIDC provider
func New(ctx context.Context, config Config) (*Provider, error) {
	wellKnown := fmt.Sprintf("%s/.well-known/openid-configuration", config.Issuer)

	// Discover endpoints
	endpoints, err := discoverEndpoints(ctx, wellKnown)
	if err != nil {
		return nil, fmt.Errorf("failed to discover endpoints: %v", err)
	}

	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  endpoints.Authorization,
			TokenURL: endpoints.Token,
		},
		Scopes: config.Scopes,
	}

	return &Provider{
		oauth2Config: oauth2Config,
		issuer:       config.Issuer,
		endpoints:    *endpoints,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// GetAuthURL returns the authorization URL
func (p *Provider) GetAuthURL(state string) string {
	return p.oauth2Config.AuthCodeURL(state)
}

// Exchange exchanges the authorization code for tokens
func (p *Provider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.oauth2Config.Exchange(ctx, code)
}

// TokenSource returns a token source that refreshes the token as needed
func (p *Provider) TokenSource(ctx context.Context, token *oauth2.Token) oauth2.TokenSource {
	return p.oauth2Config.TokenSource(ctx, token)
}

// VerifyToken verifies the JWT token and returns the claims
func (p *Provider) VerifyToken(ctx context.Context, token string) (*Claims, error) {
	parsed, err := jwt.ParseSigned(token)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	claims := &Claims{}
	if err := parsed.UnsafeClaimsWithoutVerification(claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %v", err)
	}

	// Verify issuer
	if claims.Issuer != p.issuer {
		return nil, fmt.Errorf("invalid issuer: %s", claims.Issuer)
	}

	// Verify expiration
	if claims.Expiry.Time().Before(time.Now()) {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// GetUserInfo fetches the user info from the provider
func (p *Provider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*Claims, error) {
	req, err := http.NewRequest("GET", p.endpoints.UserInfo, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

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

func discoverEndpoints(ctx context.Context, wellKnownURL string) (*Endpoints, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", wellKnownURL, nil)
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
		Authorization: discovery.AuthorizationEndpoint,
		Token:         discovery.TokenEndpoint,
		UserInfo:      discovery.UserInfoEndpoint,
		JWKS:          discovery.JWKSURI,
	}, nil
}
