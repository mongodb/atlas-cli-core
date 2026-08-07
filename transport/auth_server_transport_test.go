// Copyright 2026 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transport

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	atlasauth "go.mongodb.org/atlas/auth"
)

// recordingTransport captures the last request's Authorization header.
type recordingTransport struct {
	lastAuthHeader string
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.lastAuthHeader = req.Header.Get("Authorization")
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func TestAuthServerTransport_RoundTrip(t *testing.T) {
	token := &atlasauth.Token{
		AccessToken: "valid-at",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(time.Hour),
	}
	base := &recordingTransport{}
	tr := &authServerTransport{
		token:     token,
		base:      base,
		saveToken: func(*atlasauth.Token) error { return nil },
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.example.com/x", nil)
	require.NoError(t, err)
	resp, err := tr.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "Bearer valid-at", base.lastAuthHeader)
}

func TestAuthServerTransport_RoundTrip_Refresh(t *testing.T) {
	tokenEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"access_token":"refreshed-at","refresh_token":"new-rt","token_type":"Bearer","expires_in":3600}`)
	}))
	defer tokenEndpoint.Close()

	expired := &atlasauth.Token{
		AccessToken:  "expired-at",
		RefreshToken: "old-rt",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Minute),
	}

	authCfg, err := FlowForAuthIssuer(&fakeAuthIssuerGetter{service: "cloud"}, http.DefaultClient, testVersion)
	require.NoError(t, err)

	var saved *atlasauth.Token
	base := &recordingTransport{}
	tr := &authServerTransport{
		token:      expired,
		authConfig: authCfg,
		metadata:   map[string]any{"metadata": map[string]any{"token_endpoint": tokenEndpoint.URL}},
		base:       base,
		saveToken: func(t *atlasauth.Token) error {
			saved = t
			return nil
		},
	}

	req, err := http.NewRequest(http.MethodGet, "https://api.example.com/x", nil)
	require.NoError(t, err)
	_, err = tr.RoundTrip(req)
	require.NoError(t, err)

	require.NotNil(t, saved)
	assert.Equal(t, "refreshed-at", saved.AccessToken)
	assert.Equal(t, "Bearer refreshed-at", base.lastAuthHeader)
}

func TestAuthServerTransport_RoundTrip_MissingTokenEndpoint(t *testing.T) {
	expired := &atlasauth.Token{
		AccessToken: "expired-at",
		Expiry:      time.Now().Add(-time.Minute),
	}
	tr := &authServerTransport{
		token:     expired,
		metadata:  map[string]any{"metadata": map[string]any{}},
		base:      &recordingTransport{},
		saveToken: func(*atlasauth.Token) error { return nil },
	}

	req, _ := http.NewRequest(http.MethodGet, "https://api.example.com/x", nil)
	_, err := tr.RoundTrip(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token_endpoint not found")
}
