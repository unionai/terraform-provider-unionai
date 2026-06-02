package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestGetApiTokenUsesUnionAuthMetadata(t *testing.T) {
	originalHTTPClient := authHTTPClient
	defer func() {
		authHTTPClient = originalHTTPClient
	}()

	var tokenRequestScope string
	var tokenRequestAudience string
	var tokenRequestClientID string
	var tokenRequestClientSecret string

	authHTTPClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/.well-known/oauth-authorization-server":
				return jsonResponse(t, oauthAuthorizationServerMetadata{
					TokenEndpoint: "https://union.test/token",
				}), nil
			case "/config/v1/flyte_client":
				return jsonResponse(t, publicClientConfig{
					Scopes:                   []string{"scope-one", "scope-two"},
					Audience:                 "api://union-test",
					AuthorizationMetadataKey: "x-union-authorization",
				}), nil
			case "/token":
				if err := r.ParseForm(); err != nil {
					t.Fatalf("failed to parse token request form: %v", err)
				}
				tokenRequestScope = r.Form.Get("scope")
				tokenRequestAudience = r.Form.Get("audience")
				tokenRequestClientID, tokenRequestClientSecret, _ = r.BasicAuth()

				return jsonResponse(t, map[string]any{
					"access_token": "test-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				}), nil
			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     http.Header{},
				}, nil
			}
		}),
	}

	apiKey := encodeAPIKey("union.test", "client-id", "client-secret", "None")
	cfg, err := GetApiToken(apiKey)
	if err != nil {
		t.Fatalf("GetApiToken returned error: %v", err)
	}

	if tokenRequestScope != "scope-one scope-two" {
		t.Fatalf("expected scope to be discovered from Union metadata, got %q", tokenRequestScope)
	}
	if tokenRequestAudience != "api://union-test" {
		t.Fatalf("expected audience to be discovered from Union metadata, got %q", tokenRequestAudience)
	}
	if tokenRequestClientID != "client-id" || tokenRequestClientSecret != "client-secret" {
		t.Fatalf("unexpected client credentials: %q/%q", tokenRequestClientID, tokenRequestClientSecret)
	}
	if cfg.Host != "union.test" {
		t.Fatalf("unexpected host: %q", cfg.Host)
	}
	if cfg.AuthorizationMetadataKey != "x-union-authorization" {
		t.Fatalf("unexpected authorization metadata key: %q", cfg.AuthorizationMetadataKey)
	}

	token, err := cfg.TokenSource.Token()
	if err != nil {
		t.Fatalf("TokenSource returned error: %v", err)
	}
	if token.AccessToken != "test-access-token" {
		t.Fatalf("unexpected access token: %q", token.AccessToken)
	}
}

func TestGetApiTokenUsesDefaultAuthorizationMetadataKey(t *testing.T) {
	originalHTTPClient := authHTTPClient
	defer func() {
		authHTTPClient = originalHTTPClient
	}()

	authHTTPClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/.well-known/oauth-authorization-server":
				return jsonResponse(t, oauthAuthorizationServerMetadata{
					TokenEndpoint: "https://union.test/token",
				}), nil
			case "/config/v1/flyte_client":
				return jsonResponse(t, publicClientConfig{}), nil
			case "/token":
				return jsonResponse(t, map[string]any{
					"access_token": "test-access-token",
					"token_type":   "Bearer",
					"expires_in":   3600,
				}), nil
			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     http.Header{},
				}, nil
			}
		}),
	}

	apiKey := encodeAPIKey("union.test", "client-id", "client-secret", "None")
	cfg, err := GetApiToken(apiKey)
	if err != nil {
		t.Fatalf("GetApiToken returned error: %v", err)
	}

	if cfg.AuthorizationMetadataKey != defaultAuthorizationMetadataKey {
		t.Fatalf("unexpected authorization metadata key: %q", cfg.AuthorizationMetadataKey)
	}
}

func TestTokenSourceCredentialsUsesConfiguredHeaderKey(t *testing.T) {
	creds := NewTokenSourceCredentials(&TokenSource{
		token: &oauth2.Token{
			AccessToken: "test-token",
			TokenType:   "Bearer",
		},
	}, "x-custom-auth")

	metadata, err := creds.GetRequestMetadata(context.Background())
	if err != nil {
		t.Fatalf("GetRequestMetadata returned error: %v", err)
	}

	if metadata["x-custom-auth"] != "Bearer test-token" {
		t.Fatalf("unexpected metadata: %v", metadata)
	}
	if !creds.RequireTransportSecurity() {
		t.Fatal("expected credentials to require transport security")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(t *testing.T, value any) *http.Response {
	t.Helper()

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		t.Fatalf("failed to write JSON response: %v", err)
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&body),
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}
}

func encodeAPIKey(host, clientID, clientSecret, org string) string {
	return base64.StdEncoding.EncodeToString([]byte(strings.Join([]string{host, clientID, clientSecret, org}, ":")))
}
