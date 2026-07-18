package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/takayoshiotake/shiroyagi/internal/config"
)

func TestNewClientUsesConfiguredIssuerForAuthorizationURL(t *testing.T) {
	var issuer string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/realms/dev/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issuer": "` + issuer + `",
			"authorization_endpoint": "` + issuer + `/protocol/openid-connect/auth",
			"token_endpoint": "` + issuer + `/protocol/openid-connect/token",
			"jwks_uri": "` + issuer + `/protocol/openid-connect/certs",
			"id_token_signing_alg_values_supported": ["RS256"]
		}`))
	}))
	defer server.Close()
	issuer = server.URL + "/realms/dev"

	client, err := NewClient(t.Context(), config.Config{
		Issuer:       issuer,
		ClientID:     "shiroyagi",
		ClientSecret: "secret",
		RedirectURI:  "http://localhost:8080/auth/callback",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	authURL := client.AuthCodeURL("state", "nonce", false)
	if !strings.HasPrefix(authURL, issuer+"/protocol/openid-connect/auth") {
		t.Fatalf("AuthCodeURL() = %q, want configured issuer", authURL)
	}
}

func TestNewClientWithRetryAppliesTimeoutToDiscoveryRequest(t *testing.T) {
	block := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer server.Close()
	defer close(block)

	start := time.Now()
	_, err := NewClientWithRetry(context.Background(), config.Config{
		Issuer:       server.URL,
		ClientID:     "shiroyagi",
		ClientSecret: "secret",
		RedirectURI:  "http://localhost:8080/auth/callback",
	}, 50*time.Millisecond)
	if err == nil {
		t.Fatal("NewClientWithRetry() error = nil, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("NewClientWithRetry() error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("NewClientWithRetry() elapsed = %v, want under 1s", elapsed)
	}
}
