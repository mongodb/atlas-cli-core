// Copyright 2025 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transport

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"go.mongodb.org/atlas-sdk/v20250312027/auth/code"
	atlasauth "go.mongodb.org/atlas/auth"
)

// NewAuthCodeTransport creates a transport for Authorization Code + PKCE (user_account)
// profiles. Refresh and persistence are serialized under one mutex; unrecoverable
// refresh failures clear the profile credentials.
func NewAuthCodeTransport(profile ProfileProvider, token *atlasauth.Token, base http.RoundTripper, version string) (http.RoundTripper, error) {
	if token == nil {
		return nil, errors.New("token is nil")
	}

	flow, err := FlowForAuthServer(profile, &http.Client{Transport: Default()}, version)
	if err != nil {
		return nil, err
	}
	metadata, err := profile.AuthServerMetadata()
	if err != nil {
		return nil, err
	}

	return &authCodeTransport{
		token:    authCodeToken(token),
		flow:     flow,
		metadata: metadata,
		base:     base,
		saveToken: func(t *code.Token) error {
			profile.SetAccessToken(t.AccessToken)
			profile.SetRefreshToken(t.RefreshToken)
			profile.SetTokenExpiry(t.Expiry.Format(time.RFC3339))
			return profile.Save()
		},
		saveMetadata:     profile.SetAuthServerMetadata,
		clearCredentials: authCodeClearCredentials(profile),
	}, nil
}

type authCodeTransport struct {
	mu       sync.Mutex
	token    *code.Token
	flow     *code.Config
	metadata *code.AuthServerMetadata
	base     http.RoundTripper

	saveToken        func(*code.Token) error
	saveMetadata     func(*code.AuthServerMetadata) error
	clearCredentials func() error
}

func (tr *authCodeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := tr.ensureToken(req.Context()); err != nil {
		return nil, err
	}

	tr.mu.Lock()
	token := tr.token
	tr.mu.Unlock()
	token.SetAuthHeader(req)

	return tr.base.RoundTrip(req)
}

// ensureToken refreshes the token if invalid. It holds the transport mutex for
// the whole operation so concurrent requests cannot race the refresh or the
// persistence of the rotated token.
func (tr *authCodeTransport) ensureToken(ctx context.Context) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.token.Valid() {
		return nil
	}

	metadata, err := ensureAuthServerMetadata(ctx, tr.flow, tr.metadata, tr.saveMetadata)
	if err != nil {
		return err
	}
	tr.metadata = metadata

	token, _, err := tr.flow.RefreshAccessToken(ctx, metadata, tr.token.RefreshToken)
	if err != nil {
		logAuthServerFailure("refresh", err)
		if isUnrecoverableRefreshError(err) {
			if clearErr := tr.clearCredentials(); clearErr != nil {
				return errors.Join(err, clearErr)
			}
		}
		return err
	}
	// Retain the stored refresh token when the server does not rotate it.
	if token.RefreshToken == "" {
		token.RefreshToken = tr.token.RefreshToken
	}
	tr.token = token

	return tr.saveToken(token)
}

// RevokeUserAccountToken revokes the stored Authorization Code access token at the
// discovered revocation endpoint, for logout.
func RevokeUserAccountToken(ctx context.Context, profile ProfileProvider, version string) error {
	token, err := profile.Token()
	if err != nil {
		return err
	}
	if token == nil || token.AccessToken == "" {
		return nil
	}

	flow, err := FlowForAuthServer(profile, &http.Client{Transport: Default()}, version)
	if err != nil {
		return err
	}
	metadata, err := profile.AuthServerMetadata()
	if err != nil {
		return err
	}
	metadata, err = ensureAuthServerMetadata(ctx, flow, metadata, profile.SetAuthServerMetadata)
	if err != nil {
		return err
	}

	if _, err := flow.RevokeAuthServerToken(ctx, metadata, token.AccessToken, "access_token"); err != nil {
		logAuthServerFailure("revocation", err)
		return err
	}
	return nil
}

// ensureAuthServerMetadata returns the cached metadata, discovering fresh metadata
// when the cache is missing or expired.
func ensureAuthServerMetadata(ctx context.Context, flow *code.Config, metadata *code.AuthServerMetadata, save func(*code.AuthServerMetadata) error) (*code.AuthServerMetadata, error) {
	if metadata != nil && !metadata.CacheExpiry.IsZero() && metadata.CacheExpiry.After(time.Now()) {
		return metadata, nil
	}

	discovered, _, err := flow.DiscoverAuthServer(ctx)
	if err != nil {
		logAuthServerFailure("discovery", err)
		return nil, err
	}
	if save != nil {
		if err := save(discovered); err != nil {
			// The cache is best-effort; a persistence failure must not block the flow.
			logAuthServerFailure("discovery", err)
		}
	}
	return discovered, nil
}

// authCodeClearCredentials returns a callback that clears the stored
// Authorization Code credentials.
func authCodeClearCredentials(profile ProfileProvider) func() error {
	return func() error {
		profile.SetAccessToken("")
		profile.SetRefreshToken("")
		profile.SetTokenExpiry("")
		return profile.Save()
	}
}

// isUnrecoverableRefreshError reports whether retrying or reusing the stored
// credentials can never succeed, so they must be cleared.
func isUnrecoverableRefreshError(err error) bool {
	var oauthErr *code.OAuthError
	return errors.As(err, &oauthErr) && oauthErr.ErrorCode == "invalid_grant"
}

// logAuthServerFailure logs OAuth failures by coarse operation category only.
// Error messages carry OAuth error codes, never token or callback values.
func logAuthServerFailure(operation string, err error) {
	slog.Warn("OAuth operation failed", "operation", operation, slog.Any("error", err))
}

func authCodeToken(t *atlasauth.Token) *code.Token {
	return &code.Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		TokenType:    t.TokenType,
		Expiry:       t.Expiry,
	}
}
