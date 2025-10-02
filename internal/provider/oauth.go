package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type TokenSource struct {
	token *oauth2.Token
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	return t.token, nil
}

// discoverActualIssuer fetches the well-known OpenID configuration to find the actual issuer
// This handles cases where the server URL is a CNAME that redirects to the actual issuer
func discoverActualIssuer(serverURL string) (string, error) {
	configURL := fmt.Sprintf("https://%s/.well-known/openid-configuration", serverURL)

	resp, err := http.Get(configURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch OpenID config from %s: %w", configURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch OpenID config: HTTP %d", resp.StatusCode)
	}

	var config struct {
		Issuer string `json:"issuer"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return "", fmt.Errorf("failed to decode OpenID config: %w", err)
	}

	if config.Issuer == "" {
		return "", fmt.Errorf("issuer not found in OpenID configuration")
	}

	return config.Issuer, nil
}

// getTokenFromServer uses OpenID Connect discovery to find the token endpoint
// and then retrieves an access token using client credentials flow
// It handles CNAME redirects by discovering the actual issuer URL
func GetApiToken(serverURL, clientID, clientSecret string) (*TokenSource, error) {
	ctx := context.Background()

	// First, try to discover the actual issuer URL by fetching the well-known configuration
	actualIssuer, err := discoverActualIssuer(serverURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover actual issuer for %s: %w", serverURL, err)
	}

	// Use go-oidc to discover the OpenID Connect configuration with the actual issuer
	provider, err := oidc.NewProvider(ctx, actualIssuer)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OpenID Connect configuration from %s: %w", actualIssuer, err)
	}

	// Get the endpoint information from the provider
	endpoint := provider.Endpoint()

	// Create OAuth2 client credentials config using the discovered token endpoint
	config := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     endpoint.TokenURL,
		Scopes:       []string{},
	}

	// Get the token
	token, err := config.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	return &TokenSource{token: token}, nil
}
