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
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/atlas-sdk/v20250312027/auth/code"
	atlasauth "go.mongodb.org/atlas/auth"
	"go.uber.org/mock/gomock"
)

// authCodeTestServer is a TLS authorization server serving discovery and token responses.
type authCodeTestServer struct {
	*httptest.Server
	tokenCalls atomic.Int32
	response   func(w http.ResponseWriter, r *http.Request)
}

func newAuthCodeTestServer(t *testing.T) *authCodeTestServer {
	t.Helper()
	server := &authCodeTestServer{}
	server.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.tokenCalls.Add(1)
		server.response(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

func (s *authCodeTestServer) metadata() *code.AuthServerMetadata {
	return &code.AuthServerMetadata{
		Issuer:                s.URL,
		AuthorizationEndpoint: s.URL + "/oauth/authorize",
		TokenEndpoint:         s.URL + "/tokens",
		RevocationEndpoint:    s.URL + "/tokens/revoke",
		CacheExpiry:           time.Now().Add(time.Hour),
	}
}

func (s *authCodeTestServer) serveRefreshToken(accessToken, refreshToken string) {
	s.response = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"` + accessToken + `","token_type":"Bearer","expires_in":3600,"refresh_token":"` + refreshToken + `"}`))
	}
}

func (s *authCodeTestServer) serveRefreshError(errorCode string) {
	s.response = func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"` + errorCode + `","error_description":"mock failure"}`))
	}
}

// newTestAuthCodeTransport builds an authCodeTransport against the TLS test server,
// bypassing HTTPClientFromProfile, whose OAuth client is fixed to the default transport.
func newTestAuthCodeTransport(t *testing.T, server *authCodeTestServer, token *atlasauth.Token, saveToken func(*code.Token, *code.AuthServerMetadata) error, clearCredentials func() error) *authCodeTransport {
	t.Helper()

	flow, err := code.NewConfigWithOptions(server.Client(),
		code.SetAuthServerURL(server.URL),
		code.SetClientID("test-client"),
		code.SetScopes([]string{"atlas"}),
	)
	require.NoError(t, err)

	metadata := &code.AuthServerMetadata{
		Issuer:                server.URL,
		AuthorizationEndpoint: server.URL + "/oauth/authorize",
		TokenEndpoint:         server.URL + "/tokens",
		RevocationEndpoint:    server.URL + "/tokens/revoke",
		CacheExpiry:           time.Now().Add(time.Hour),
	}

	return &authCodeTransport{
		token:            authCodeToken(token),
		flow:             flow,
		metadata:         metadata,
		base:             roundTripperStub{},
		saveToken:        saveToken,
		clearCredentials: clearCredentials,
		loadRefreshToken: func() (string, error) { return token.RefreshToken, nil },
	}
}

func expiredAuthCodeToken() *atlasauth.Token {
	return &atlasauth.Token{
		AccessToken:  "old-access-token",
		RefreshToken: "old-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
}

type roundTripperStub struct{}

func (roundTripperStub) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: http.Header{}, Request: req}, nil
}

// TestAuthCodeTransportRefresh verifies the transport refreshes an expired token and
// persists the rotated tokens with an absolute expiry.
func TestAuthCodeTransportRefresh(t *testing.T) {
	server := newAuthCodeTestServer(t)
	server.serveRefreshToken("new-access-token", "new-refresh-token")

	var saved *code.Token
	tr := newTestAuthCodeTransport(t, server, expiredAuthCodeToken(),
		func(token *code.Token, _ *code.AuthServerMetadata) error { saved = token; return nil },
		func() error { t.Error("credentials must not be cleared on successful refresh"); return nil },
	)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.mongodb.com", http.NoBody)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, 1, int(server.tokenCalls.Load()), "expected exactly one refresh")
	require.NotNil(t, saved)
	assert.Equal(t, "new-access-token", saved.AccessToken)
	assert.Equal(t, "new-refresh-token", saved.RefreshToken)
	assert.WithinDuration(t, time.Now().Add(time.Hour), saved.Expiry, time.Minute)
	assert.Equal(t, "Bearer new-access-token", req.Header.Get("Authorization"))
}

// TestAuthCodeTransportRefreshKeepsUnrotatedRefreshToken verifies the stored refresh
// token is retained when the server does not rotate it.
func TestAuthCodeTransportRefreshKeepsUnrotatedRefreshToken(t *testing.T) {
	server := newAuthCodeTestServer(t)
	server.serveRefreshToken("new-access-token", "")

	var saved *code.Token
	tr := newTestAuthCodeTransport(t, server, expiredAuthCodeToken(),
		func(token *code.Token, _ *code.AuthServerMetadata) error { saved = token; return nil },
		func() error { t.Error("credentials must not be cleared on successful refresh"); return nil },
	)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.mongodb.com", http.NoBody)
	require.NoError(t, err)
	_, err = tr.RoundTrip(req)
	require.NoError(t, err)

	require.NotNil(t, saved)
	assert.Equal(t, "old-refresh-token", saved.RefreshToken)
}

// TestAuthCodeTransportRefreshSerialized verifies concurrent requests trigger a
// single serialized refresh.
func TestAuthCodeTransportRefreshSerialized(t *testing.T) {
	server := newAuthCodeTestServer(t)
	server.serveRefreshToken("new-access-token", "new-refresh-token")

	var saves atomic.Int32
	tr := newTestAuthCodeTransport(t, server, expiredAuthCodeToken(),
		func(*code.Token, *code.AuthServerMetadata) error { saves.Add(1); return nil },
		func() error { t.Error("credentials must not be cleared on successful refresh"); return nil },
	)

	const requests = 8
	var wg sync.WaitGroup
	for range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.mongodb.com", http.NoBody)
			assert.NoError(t, err)
			_, err = tr.RoundTrip(req)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, int(server.tokenCalls.Load()), "expected exactly one serialized refresh")
	assert.Equal(t, 1, int(saves.Load()), "expected exactly one token persistence")
}

// TestAuthCodeTransportUnrecoverableRefreshClearsCredentials verifies that an
// invalid_grant refresh failure clears the stored credentials, and that the failure
// log only carries a coarse operation category with no OAuth values.
func TestAuthCodeTransportUnrecoverableRefreshClearsCredentials(t *testing.T) {
	server := newAuthCodeTestServer(t)
	server.serveRefreshError("invalid_grant")

	var logBuf bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	clears := 0
	tr := newTestAuthCodeTransport(t, server, expiredAuthCodeToken(),
		func(*code.Token, *code.AuthServerMetadata) error {
			t.Error("token must not be saved on failed refresh")
			return nil
		},
		func() error { clears++; return nil },
	)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.mongodb.com", http.NoBody)
	require.NoError(t, err)
	_, err = tr.RoundTrip(req)
	require.Error(t, err)
	assert.Equal(t, 1, clears, "invalid_grant must clear credentials")

	// Later requests fail without refreshing or clearing again.
	_, err = tr.RoundTrip(req)
	require.ErrorIs(t, err, ErrNotLoggedIn)
	assert.Equal(t, 1, int(server.tokenCalls.Load()))
	assert.Equal(t, 1, clears)

	logs := logBuf.String()
	assert.Contains(t, logs, "operation=refresh")
	assert.NotContains(t, logs, "old-access-token")
	assert.NotContains(t, logs, "old-refresh-token")
}

// TestAuthCodeTransportRecoverableRefreshErrorKeepsCredentials verifies that a
// transient refresh failure does not clear the stored credentials.
func TestAuthCodeTransportRecoverableRefreshErrorKeepsCredentials(t *testing.T) {
	server := newAuthCodeTestServer(t)
	server.serveRefreshError("temporarily_unavailable")

	cleared := false
	tr := newTestAuthCodeTransport(t, server, expiredAuthCodeToken(),
		func(*code.Token, *code.AuthServerMetadata) error {
			t.Error("token must not be saved on failed refresh")
			return nil
		},
		func() error { cleared = true; return nil },
	)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.mongodb.com", http.NoBody)
	require.NoError(t, err)
	_, err = tr.RoundTrip(req)
	require.Error(t, err)
	assert.False(t, cleared, "a transient error must not clear credentials")
}

// TestEnsureAuthServerMetadata verifies stale metadata triggers discovery with the
// refreshed metadata persisted, and fresh metadata is reused without discovery.
func TestEnsureAuthServerMetadata(t *testing.T) {
	server := newAuthCodeTestServer(t)
	server.serveRefreshToken("unused", "unused")

	var discoveryCalls atomic.Int32

	t.Run("fresh metadata is reused", func(t *testing.T) {
		flow, err := code.NewConfigWithOptions(server.Client(), code.SetAuthServerURL(server.URL))
		require.NoError(t, err)

		metadata := &code.AuthServerMetadata{Issuer: server.URL, CacheExpiry: time.Now().Add(time.Hour)}
		result, err := ensureAuthServerMetadata(t.Context(), flow, metadata)
		require.NoError(t, err)
		assert.Same(t, metadata, result)
		assert.Equal(t, 0, int(discoveryCalls.Load()))
	})

	t.Run("metadata issued by another server is rediscovered", func(t *testing.T) {
		before := discoveryCalls.Load()
		flow, err := code.NewConfigWithOptions(server.Client(), code.SetAuthServerURL(server.URL))
		require.NoError(t, err)

		server.response = func(w http.ResponseWriter, _ *http.Request) {
			discoveryCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "max-age=3600")
			_, _ = w.Write([]byte(`{"issuer":"` + server.URL + `","authorization_endpoint":"` + server.URL + `/oauth/authorize","token_endpoint":"` + server.URL + `/tokens","revocation_endpoint":"` + server.URL + `/tokens/revoke"}`))
		}

		metadata := &code.AuthServerMetadata{Issuer: "https://other.example.com", CacheExpiry: time.Now().Add(time.Hour)}
		result, err := ensureAuthServerMetadata(t.Context(), flow, metadata)
		require.NoError(t, err)
		assert.Equal(t, server.URL, result.Issuer)
		assert.Equal(t, 1, int(discoveryCalls.Load()-before))
	})

	t.Run("stale metadata is not used when discovery fails", func(t *testing.T) {
		flow, err := code.NewConfigWithOptions(server.Client(), code.SetAuthServerURL(server.URL))
		require.NoError(t, err)

		server.response = func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}

		stale := &code.AuthServerMetadata{
			Issuer:                server.URL,
			AuthorizationEndpoint: server.URL + "/oauth/authorize",
			TokenEndpoint:         server.URL + "/tokens",
			CacheExpiry:           time.Now().Add(-time.Hour),
		}
		result, err := ensureAuthServerMetadata(t.Context(), flow, stale)
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("stale metadata is rediscovered", func(t *testing.T) {
		before := discoveryCalls.Load()
		server.response = func(w http.ResponseWriter, _ *http.Request) {
			discoveryCalls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "max-age=3600")
			_, _ = w.Write([]byte(`{"issuer":"` + server.URL + `","authorization_endpoint":"` + server.URL + `/oauth/authorize","token_endpoint":"` + server.URL + `/tokens","revocation_endpoint":"` + server.URL + `/tokens/revoke"}`))
		}

		flow, err := code.NewConfigWithOptions(server.Client(), code.SetAuthServerURL(server.URL))
		require.NoError(t, err)

		metadata := &code.AuthServerMetadata{Issuer: server.URL, CacheExpiry: time.Now().Add(-time.Hour)}
		result, err := ensureAuthServerMetadata(t.Context(), flow, metadata)
		require.NoError(t, err)

		require.NotNil(t, result)
		assert.Equal(t, server.URL+"/tokens", result.TokenEndpoint)
		assert.False(t, result.CacheExpiry.IsZero())
		assert.Equal(t, 1, int(discoveryCalls.Load()-before))
	})
}

func TestRevokeUserAccountToken(t *testing.T) {
	t.Run("revokes the refresh token before the access token", func(t *testing.T) {
		server := newAuthCodeTestServer(t)

		var revoked []string
		server.response = func(w http.ResponseWriter, r *http.Request) {
			assert.NoError(t, r.ParseForm())
			if r.ParseForm() != nil {
				return
			}
			revoked = append(revoked, r.Form.Get("token_type_hint")+":"+r.Form.Get("token"))
			w.WriteHeader(http.StatusOK)
		}

		flow, err := code.NewConfigWithOptions(server.Client(),
			code.SetAuthServerURL(server.URL),
			code.SetClientID("test-client"),
			code.SetScopes([]string{"atlas"}),
		)
		require.NoError(t, err)

		err = revokeAuthServerTokens(t.Context(), flow, server.metadata(), "access-token", "refresh-token")
		require.NoError(t, err)
		assert.Equal(t, []string{"refresh_token:refresh-token", "access_token:access-token"}, revoked)
	})

	t.Run("without tokens it is a no-op", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockProfile := NewMockProfileProvider(ctrl)
		mockProfile.EXPECT().AccessToken().Return("")
		mockProfile.EXPECT().RefreshToken().Return("")

		err := RevokeUserAccountToken(t.Context(), mockProfile, testVersion)
		require.NoError(t, err)
	})
}

// TestAuthCodeTransportRefreshAfterRotation verifies a refresh token rotated by
// another process is retried with the stored value instead of clearing credentials.
func TestAuthCodeTransportRefreshAfterRotation(t *testing.T) {
	server := newAuthCodeTestServer(t)
	rotated := &code.Token{
		AccessToken:  "rotated-access-token",
		RefreshToken: "rotated-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	var usedRefreshTokens []string
	server.response = func(w http.ResponseWriter, r *http.Request) {
		if !assert.NoError(t, r.ParseForm()) {
			return
		}
		usedRefreshTokens = append(usedRefreshTokens, r.Form.Get("refresh_token"))
		if r.Form.Get("refresh_token") == "old-refresh-token" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"` + rotated.AccessToken + `","token_type":"Bearer","expires_in":3600,"refresh_token":"` + rotated.RefreshToken + `"}`))
	}

	var saved *code.Token
	cleared := false
	tr := newTestAuthCodeTransport(t, server, expiredAuthCodeToken(),
		func(token *code.Token, _ *code.AuthServerMetadata) error { saved = token; return nil },
		func() error { cleared = true; return nil },
	)
	tr.loadRefreshToken = func() (string, error) { return "rotated-refresh-token", nil }

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.mongodb.com", http.NoBody)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, []string{"old-refresh-token", "rotated-refresh-token"}, usedRefreshTokens)
	assert.False(t, cleared, "credentials must not be cleared when another process rotated the token")
	require.NotNil(t, saved)
	assert.Equal(t, rotated.AccessToken, saved.AccessToken)
}

// TestRevokeAuthServerTokensPartialFailure verifies the access token is revoked
// even when the refresh token revocation fails.
func TestRevokeAuthServerTokensPartialFailure(t *testing.T) {
	server := newAuthCodeTestServer(t)

	var revoked []string
	server.response = func(w http.ResponseWriter, r *http.Request) {
		if !assert.NoError(t, r.ParseForm()) {
			return
		}
		if r.Form.Get("token_type_hint") == "refresh_token" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		revoked = append(revoked, r.Form.Get("token_type_hint")+":"+r.Form.Get("token"))
		w.WriteHeader(http.StatusOK)
	}

	flow, err := code.NewConfigWithOptions(server.Client(),
		code.SetAuthServerURL(server.URL),
		code.SetClientID("test-client"),
		code.SetScopes([]string{"atlas"}),
	)
	require.NoError(t, err)

	err = revokeAuthServerTokens(t.Context(), flow, server.metadata(), "access-token", "refresh-token")
	require.Error(t, err)
	assert.Equal(t, []string{"access_token:access-token"}, revoked)
}

// TestAuthCodeTransportStoreReadFailureKeepsCredentials verifies that when the
// stored refresh token cannot be read, an invalid_grant does not clear credentials
// another process may have rotated.
func TestAuthCodeTransportStoreReadFailureKeepsCredentials(t *testing.T) {
	server := newAuthCodeTestServer(t)
	server.serveRefreshError("invalid_grant")

	tr := newTestAuthCodeTransport(t, server, expiredAuthCodeToken(),
		func(*code.Token, *code.AuthServerMetadata) error {
			t.Error("token must not be saved on failed refresh")
			return nil
		},
		func() error { t.Error("credentials must not be cleared when the store cannot be read"); return nil },
	)
	tr.loadRefreshToken = func() (string, error) { return "", errors.New("keychain locked") }

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://example.mongodb.com", http.NoBody)
	require.NoError(t, err)
	_, err = tr.RoundTrip(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keychain locked")
}

// TestAuthCodeClearCredentials verifies clearing removes the issuer binding along
// with the tokens, so MONGODB_ATLAS_AUTH_SERVER_URL applies to the next login.
func TestAuthCodeClearCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockProfile := NewMockProfileProvider(ctrl)
	mockProfile.EXPECT().SetAccessToken("")
	mockProfile.EXPECT().SetRefreshToken("")
	mockProfile.EXPECT().SetTokenExpiry("")
	mockProfile.EXPECT().SetAuthServerIssuer("")
	mockProfile.EXPECT().SetAuthServerMetadata(nil)
	mockProfile.EXPECT().Save().Return(nil)

	require.NoError(t, authCodeClearCredentials(mockProfile)())
}
