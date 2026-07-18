package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/takayoshiotake/shiroyagi/internal/config"
	"golang.org/x/oauth2"
)

type Client struct {
	oauth2Config oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

type IDTokenClaims struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	AuthTime          int64  `json:"auth_time"`
}

func NewClient(ctx context.Context, cfg config.Config) (*Client, error) {
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}

	endpoint := provider.Endpoint()

	return &Client{
		oauth2Config: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     endpoint,
			RedirectURL:  cfg.RedirectURI,
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: provider.Verifier(&oidc.Config{
			ClientID: cfg.ClientID,
		}),
	}, nil
}

func NewClientWithRetry(ctx context.Context, cfg config.Config, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		return NewClient(ctx, cfg)
	}

	retryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var lastErr error
	for {
		client, err := NewClient(retryCtx, cfg)
		if err == nil {
			return client, nil
		}
		lastErr = err

		if err := retryCtx.Err(); err != nil {
			return nil, fmt.Errorf("%w: last OIDC discovery error: %v", err, lastErr)
		}

		timer := time.NewTimer(time.Second)
		select {
		case <-retryCtx.Done():
			timer.Stop()
			return nil, fmt.Errorf(
				"%w: last OIDC discovery error: %v",
				retryCtx.Err(),
				lastErr,
			)
		case <-timer.C:
		}
	}
}

func (c *Client) AuthCodeURL(state, nonce string, forceReauth bool) string {
	authCodeOptions := []oauth2.AuthCodeOption{oidc.Nonce(nonce)}
	if forceReauth {
		authCodeOptions = append(authCodeOptions, oauth2.SetAuthURLParam("max_age", "0"))
	}
	return c.oauth2Config.AuthCodeURL(state, authCodeOptions...)
}

func (c *Client) ExchangeCode(ctx context.Context, code, expectedNonce string) (UserSession, error) {
	token, err := c.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return UserSession{}, fmt.Errorf("exchange OIDC code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return UserSession{}, errors.New("missing ID token")
	}

	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return UserSession{}, fmt.Errorf("verify ID token: %w", err)
	}
	if idToken.Nonce != expectedNonce {
		return UserSession{}, errors.New("invalid OIDC nonce")
	}

	var claims IDTokenClaims
	if err := idToken.Claims(&claims); err != nil {
		return UserSession{}, fmt.Errorf("read ID token claims: %w", err)
	}
	return UserSession{
		Subject:           claims.Subject,
		Email:             claims.Email,
		Name:              claims.Name,
		PreferredUsername: claims.PreferredUsername,
		AuthTime:          claims.AuthTime,
		CreatedAt:         time.Now(),
	}, nil
}
