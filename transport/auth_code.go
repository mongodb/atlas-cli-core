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
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/mongodb/atlas-cli-core/config"
	"go.mongodb.org/atlas-sdk/v20250312027/auth/code"
	atlasauth "go.mongodb.org/atlas/auth"
)

// ErrNotLoggedIn is returned once the profile's credentials were cleared after an
// unrecoverable refresh failure.
var ErrNotLoggedIn = errors.New("not logged in, run `atlas auth login`")

var errNotUserAccount = errors.New("not a user_account profile")

// NewAuthCodeTransport creates a transport for Authorization Code + PKCE (user_account)
// profiles. Refresh and persistence are serialized under one mutex; unrecoverable
// refresh failures clear the profile credentials.
func NewAuthCodeTransport(profile ProfileProvider, token *atlasauth.Token, base http.RoundTripper, version string) (http.RoundTripper, error) {
	if token == nil {
		return nil, errors.New("token is nil")
	}

	refresher, err := newAuthCodeRefresher(profile, &http.Client{Transport: Default()}, version)
	if err != nil {
		return nil, err
	}

	return &authCodeTransport{
		token:     authCodeToken(token),
		refresher: refresher,
		base:      base,
	}, nil
}

// SaveUserAccountToken persists an Authorization Code token, its expiry, and its
// issuer binding. It is the single write path for user_account credentials, used
// by both the refresh transport and the CLI login flow.
func SaveUserAccountToken(profile ProfileProvider, t *code.Token, metadata *code.AuthServerMetadata) error {
	profile.SetAccessToken(t.AccessToken)
	profile.SetRefreshToken(t.RefreshToken)
	profile.SetTokenExpiry(t.Expiry.Format(time.RFC3339))
	if metadata != nil {
		profile.SetAuthServerIssuer(metadata.Issuer)
		profile.SetAuthServerMetadata(metadata)
	}
	return profile.Save()
}

type authCodeTransport struct {
	mu        sync.Mutex
	token     *code.Token
	refresher *authCodeRefresher
	base      http.RoundTripper
}

func (tr *authCodeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := tr.ensureToken(req.Context())
	if err != nil {
		return nil, err
	}
	token.SetAuthHeader(req)

	return tr.base.RoundTrip(req)
}

// ensureToken returns a valid token, refreshing it if needed. It holds the
// transport mutex for the whole operation so concurrent requests cannot race the
// refresh or the persistence of the rotated token.
func (tr *authCodeTransport) ensureToken(ctx context.Context) (*code.Token, error) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.token == nil {
		return nil, ErrNotLoggedIn
	}
	if tr.token.Valid() {
		return tr.token, nil
	}

	token, dead, err := tr.refresher.refresh(ctx, tr.token)
	if dead {
		tr.token = nil
	} else if token != nil {
		tr.token = token
	}
	return token, err
}

// authCodeRefresher is the single refresh path for user_account credentials,
// shared by authCodeTransport and RefreshUserAccountToken. It is not safe for
// concurrent use; callers serialize access.
type authCodeRefresher struct {
	flow     *code.Config
	metadata *code.AuthServerMetadata

	saveToken        func(*code.Token, *code.AuthServerMetadata) error
	clearCredentials func() error
	loadRefreshToken func() (string, error)
}

func newAuthCodeRefresher(profile ProfileProvider, client *http.Client, version string) (*authCodeRefresher, error) {
	flow, err := FlowForAuthServer(profile, client, version)
	if err != nil {
		return nil, err
	}

	return &authCodeRefresher{
		flow:     flow,
		metadata: profile.AuthServerMetadata(),
		saveToken: func(t *code.Token, md *code.AuthServerMetadata) error {
			return SaveUserAccountToken(profile, t, md)
		},
		clearCredentials: authCodeClearCredentials(profile),
		loadRefreshToken: profile.FreshRefreshToken,
	}, nil
}

// refresh exchanges current's refresh token and persists the result. When
// persisting fails the refreshed token is still returned with the error: the
// server may have rotated the refresh token, so it is the only usable one. dead
// reports that the credentials can never refresh and were cleared; the error
// then wraps ErrNotLoggedIn.
func (r *authCodeRefresher) refresh(ctx context.Context, current *code.Token) (token *code.Token, dead bool, err error) {
	metadata, err := ResolveAuthServerMetadata(ctx, r.flow, r.metadata, false)
	if err != nil {
		return nil, false, err
	}
	r.metadata = metadata

	token, dead, err = r.refreshToken(ctx, metadata, current.RefreshToken)
	if err != nil {
		logAuthServerFailure("refresh", err)
		if dead {
			err = errors.Join(err, ErrNotLoggedIn)
			if clearErr := r.clearCredentials(); clearErr != nil {
				err = errors.Join(err, clearErr)
			}
		}
		return nil, dead, err
	}

	return token, false, r.saveToken(token, metadata)
}

// refreshToken retries once with the stored refresh token when it was rotated
// by another process. dead reports that the stored credentials can never refresh:
// the stored refresh token was read and was rejected too, or is not newer.
func (r *authCodeRefresher) refreshToken(ctx context.Context, metadata *code.AuthServerMetadata, used string) (token *code.Token, dead bool, err error) {
	token, _, err = r.flow.RefreshAccessToken(ctx, metadata, used)
	if err != nil && isUnrecoverableRefreshError(err) {
		stored, loadErr := r.loadRefreshToken()
		if loadErr != nil {
			return nil, false, errors.Join(err, loadErr)
		}
		if stored == "" || stored == used {
			return nil, true, err
		}
		used = stored
		token, _, err = r.flow.RefreshAccessToken(ctx, metadata, used)
		if err != nil {
			return nil, isUnrecoverableRefreshError(err), err
		}
	}
	if err != nil {
		return nil, false, err
	}
	if token.RefreshToken == "" {
		token.RefreshToken = used
	}
	return token, false, nil
}

// RefreshUserAccountToken refreshes the stored Authorization Code token of a
// user_account profile when it is expired, for callers that need a valid token
// outside an HTTP request. It shares its refresh path with the transport: an
// unrecoverable refresh failure clears the profile credentials and returns an
// error wrapping ErrNotLoggedIn.
func RefreshUserAccountToken(ctx context.Context, profile ProfileProvider, version string) error {
	return refreshUserAccountTokenWithClient(ctx, profile, &http.Client{Transport: Default()}, version)
}

func refreshUserAccountTokenWithClient(ctx context.Context, profile ProfileProvider, client *http.Client, version string) error {
	if authType := profile.AuthType(); authType != config.UserAccount {
		return fmt.Errorf("%w: profile auth type is %q", errNotUserAccount, authType)
	}

	token, err := profile.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return ErrNotLoggedIn
	}
	current := authCodeToken(token)
	if current.Valid() {
		return nil
	}

	refresher, err := newAuthCodeRefresher(profile, client, version)
	if err != nil {
		return err
	}
	_, _, err = refresher.refresh(ctx, current)
	return err
}

// RevokeUserAccountToken revokes the stored Authorization Code tokens at the
// discovered revocation endpoint, for logout. The refresh token is revoked first:
// it is the long-lived credential. Callers must clear the profile credentials
// afterward, regardless of the revocation outcome.
func RevokeUserAccountToken(ctx context.Context, profile ProfileProvider, version string) error {
	if profile.AccessToken() == "" && profile.RefreshToken() == "" {
		return nil
	}

	flow, err := FlowForAuthServer(profile, &http.Client{Transport: Default()}, version)
	if err != nil {
		return err
	}
	metadata, err := ResolveAuthServerMetadata(ctx, flow, profile.AuthServerMetadata(), false)
	if err != nil {
		return err
	}

	return revokeAuthServerTokens(ctx, flow, metadata, profile.AccessToken(), profile.RefreshToken())
}

func revokeAuthServerTokens(ctx context.Context, flow *code.Config, metadata *code.AuthServerMetadata, accessToken, refreshToken string) error {
	var errs []error
	if refreshToken != "" {
		if _, err := flow.RevokeAuthServerToken(ctx, metadata, refreshToken, "refresh_token"); err != nil {
			logAuthServerFailure("revocation", err)
			errs = append(errs, err)
		}
	}
	if accessToken != "" {
		if _, err := flow.RevokeAuthServerToken(ctx, metadata, accessToken, "access_token"); err != nil {
			logAuthServerFailure("revocation", err)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ResolveAuthServerMetadata returns the cached metadata, discovering fresh metadata
// when forceRefresh is set or the cache is missing, expired, or issued by a different
// authorization server than flow's. A cache without an expiry is treated as expired.
// The caller persists the result, for example with SaveUserAccountToken.
func ResolveAuthServerMetadata(ctx context.Context, flow *code.Config, cached *code.AuthServerMetadata, forceRefresh bool) (*code.AuthServerMetadata, error) {
	if !forceRefresh && cached != nil && cached.Issuer == flow.AuthServerURL.String() &&
		cached.CacheExpiry.After(time.Now()) {
		return cached, nil
	}

	discovered, _, err := flow.DiscoverAuthServer(ctx)
	if err != nil {
		logAuthServerFailure("discovery", err)
		return nil, err
	}
	return discovered, nil
}

// authCodeClearCredentials returns a callback that clears the stored
// Authorization Code credentials and the issuer binding they belong to.
func authCodeClearCredentials(profile ProfileProvider) func() error {
	return func() error {
		profile.SetAccessToken("")
		profile.SetRefreshToken("")
		profile.SetTokenExpiry("")
		profile.SetAuthServerIssuer("")
		profile.SetAuthServerMetadata(nil)
		return profile.Save()
	}
}

// isUnrecoverableRefreshError reports whether the server rejected the refresh token.
func isUnrecoverableRefreshError(err error) bool {
	var oauthErr *code.OAuthError
	return errors.As(err, &oauthErr) && oauthErr.ErrorCode == "invalid_grant"
}

// logAuthServerFailure logs an OAuth failure with its operation category. Errors
// carry OAuth error codes and descriptions, never token or callback values.
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
