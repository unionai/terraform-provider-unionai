package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const defaultAuthorizationMetadataKey = "authorization"

var authHTTPClient = http.DefaultClient

type TokenSource struct {
	token *oauth2.Token
}

func (t *TokenSource) Token() (*oauth2.Token, error) {
	return t.token, nil
}

type TokenSourceCredentials struct {
	tokenSource oauth2.TokenSource
	headerKey   string
}

func NewTokenSourceCredentials(tokenSource oauth2.TokenSource, headerKey string) TokenSourceCredentials {
	if headerKey == "" {
		headerKey = defaultAuthorizationMetadataKey
	}
	return TokenSourceCredentials{
		tokenSource: tokenSource,
		headerKey:   headerKey,
	}
}

func (c TokenSourceCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	token, err := c.tokenSource.Token()
	if err != nil {
		return nil, err
	}

	return map[string]string{
		c.headerKey: token.Type() + " " + token.AccessToken,
	}, nil
}

func (c TokenSourceCredentials) RequireTransportSecurity() bool {
	return true
}

type ApiTokenConfig struct {
	TokenSource              *TokenSource
	Host                     string
	Org                      string
	AuthorizationMetadataKey string
	Scopes                   []string
	Audience                 string
}

type oauthAuthorizationServerMetadata struct {
	Issuer             string `json:"issuer"`
	TokenEndpoint      string `json:"token_endpoint"`
	TokenEndpointCamel string `json:"tokenEndpoint"`
}

type publicClientConfig struct {
	Scopes                            []string `json:"scopes"`
	Audience                          string   `json:"audience"`
	AuthorizationMetadataKey          string   `json:"authorization_metadata_key"`
	AuthorizationMetadataKeyCamelCase string   `json:"authorizationMetadataKey"`
}

func (m oauthAuthorizationServerMetadata) getTokenEndpoint() string {
	if m.TokenEndpoint != "" {
		return m.TokenEndpoint
	}
	return m.TokenEndpointCamel
}

func (c publicClientConfig) getAuthorizationMetadataKey() string {
	if c.AuthorizationMetadataKey != "" {
		return c.AuthorizationMetadataKey
	}
	return c.AuthorizationMetadataKeyCamelCase
}

func authMetadataURL(host, path string) string {
	normalizedHost := strings.TrimRight(strings.TrimPrefix(host, "dns:///"), "/")
	if strings.HasPrefix(normalizedHost, "http://") || strings.HasPrefix(normalizedHost, "https://") {
		return normalizedHost + path
	}
	return fmt.Sprintf("https://%s%s", normalizedHost, path)
}

func fetchJSON(url string, out any) error {
	resp, err := authHTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch %s: HTTP %d", url, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode %s: %w", url, err)
	}

	return nil
}

func discoverUnionAuthMetadata(host string) (*oauthAuthorizationServerMetadata, *publicClientConfig, error) {
	var oauthMetadata oauthAuthorizationServerMetadata
	if err := fetchJSON(authMetadataURL(host, "/.well-known/oauth-authorization-server"), &oauthMetadata); err != nil {
		return nil, nil, err
	}
	if oauthMetadata.getTokenEndpoint() == "" {
		return nil, nil, fmt.Errorf("token_endpoint not found in OAuth authorization server metadata")
	}

	var clientConfig publicClientConfig
	if err := fetchJSON(authMetadataURL(host, "/config/v1/flyte_client"), &clientConfig); err != nil {
		return nil, nil, err
	}

	return &oauthMetadata, &clientConfig, nil
}

// discoverActualIssuer fetches the well-known OpenID configuration to find the actual issuer
// This handles cases where the server URL is a CNAME that redirects to the actual issuer
func discoverActualIssuer(serverURL string) (string, error) {
	configURL := authMetadataURL(serverURL, "/.well-known/openid-configuration")

	var config struct {
		Issuer string `json:"issuer"`
	}
	if err := fetchJSON(configURL, &config); err != nil {
		return "", err
	}

	if config.Issuer == "" {
		return "", fmt.Errorf("issuer not found in OpenID configuration")
	}

	return config.Issuer, nil
}

// getTokenFromServer uses OpenID Connect discovery to find the token endpoint
// and then retrieves an access token using client credentials flow
// It handles CNAME redirects by discovering the actual issuer URL
func GetApiToken(apiKey string) (*ApiTokenConfig, error) {
	ctx := context.Background()

	// Decode API key
	host, clientID, clientSecret, org, err := decodeApiKey(apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode API key: %w", err)
	}
	if org == "" || strings.EqualFold(org, "none") {
		org = orgFromHost(host)
	}

	tokenURL := ""
	scopes := []string{}
	audience := ""
	authorizationMetadataKey := defaultAuthorizationMetadataKey

	oauthMetadata, clientConfig, err := discoverUnionAuthMetadata(host)
	if err == nil {
		tokenURL = oauthMetadata.getTokenEndpoint()
		scopes = clientConfig.Scopes
		audience = clientConfig.Audience
		if clientConfig.getAuthorizationMetadataKey() != "" {
			authorizationMetadataKey = clientConfig.getAuthorizationMetadataKey()
		}
	} else {
		// Fall back to the older OIDC discovery behavior for deployments that do
		// not expose Union client metadata.
		actualIssuer, issuerErr := discoverActualIssuer(host)
		if issuerErr != nil {
			return nil, fmt.Errorf("failed to discover Union auth metadata for %s: %w; failed to discover actual issuer: %w", host, err, issuerErr)
		}

		provider, providerErr := oidc.NewProvider(context.WithValue(ctx, oauth2.HTTPClient, authHTTPClient), actualIssuer)
		if providerErr != nil {
			return nil, fmt.Errorf("failed to discover OpenID Connect configuration from %s: %w", actualIssuer, providerErr)
		}

		tokenURL = provider.Endpoint().TokenURL
	}

	if tokenURL == "" {
		return nil, fmt.Errorf("token endpoint not found")
	}

	tokenCtx := context.WithValue(ctx, oauth2.HTTPClient, authHTTPClient)
	config := clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		Scopes:       scopes,
	}
	if audience != "" {
		config.EndpointParams = url.Values{
			"audience": []string{audience},
		}
	}
	// Get the token
	token, err := config.Token(tokenCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	return &ApiTokenConfig{
		TokenSource:              &TokenSource{token: token},
		Host:                     host,
		Org:                      org,
		AuthorizationMetadataKey: authorizationMetadataKey,
		Scopes:                   scopes,
		Audience:                 audience,
	}, nil
}

func decodeApiKey(apiKey string) (string, string, string, string, error) {
	// base64 decode the key
	decodedKey, err := base64.StdEncoding.DecodeString(apiKey)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to decode API key: %w", err)
	}

	// API key format: serverURL:clientID:clientSecret:org
	parts := strings.SplitN(string(decodedKey), ":", 4)
	if len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("invalid API key format")
	}

	return parts[0], parts[1], parts[2], parts[3], nil
}

func orgFromHost(host string) string {
	normalizedHost := strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "dns:///")
	return strings.Split(normalizedHost, ".")[0]
}
