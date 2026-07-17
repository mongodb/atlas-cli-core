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

package e2e

import (
	"net/http"
	"testing"

	"github.com/mongodb/atlas-cli-core/config"
	"github.com/mongodb/atlas-cli-core/test/internal"
	"github.com/mongodb/atlas-cli-core/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	atlasauth "go.mongodb.org/atlas/auth"
	"golang.org/x/oauth2"
)

// TestDigestTransport tests the digest authentication transport through an actual Atlas API call.
func TestDigestTransport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	tc := internal.NewTestCredentials(t)

	digestTransport := transport.NewDigestTransport(tc.PublicKey, tc.PrivateKey, transport.Default())

	url := tc.BaseURL + "api/atlas/v2/orgs"
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", internal.AcceptHeader)

	resp, err := digestTransport.RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "Should get successful response")
	t.Logf("Digest authentication successful, status: %d", resp.StatusCode)
}

// TestServiceAccountTransport tests the service account transport through an actual Atlas API call.
func TestServiceAccountTransport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	tc := internal.NewTestCredentials(t)
	tc.CreateServiceAccount()

	tmp := config.OpsManagerURL()
	config.SetOpsManagerURL(tc.BaseURL)
	defer config.SetOpsManagerURL(tmp)
	// Test the service account client from transport package
	client := transport.NewServiceAccountClientWithHost(t.Context(), tc.ClientID, tc.ClientSecret, tc.BaseURL, "e2e-test", nil, func(_ *oauth2.Token) error { return nil })
	require.NotNil(t, client)

	// Test actual API call using the service account
	url := tc.BaseURL + "api/atlas/v2/orgs"
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", internal.AcceptHeader)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()

	assert.Equal(t, 200, resp.StatusCode, "Should get successful response with service account")
	t.Logf("Service account authentication successful, status: %d", resp.StatusCode)
}

// TestServiceAccountTokenReuse verifies that a persisted service-account token is reused across
// separate client constructions (each construction simulates a new CLI invocation), so repeated
// API calls do not mint a new token every time.
func TestServiceAccountTokenReuse(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	tc := internal.NewTestCredentials(t)
	tc.CreateServiceAccount()

	tmp := config.OpsManagerURL()
	config.SetOpsManagerURL(tc.BaseURL)
	defer config.SetOpsManagerURL(tmp)

	const version = "e2e-test"
	url := tc.BaseURL + "api/atlas/v2/orgs"

	// doCall performs an authenticated GET and asserts a 200, mirroring a single CLI command.
	doCall := func(t *testing.T, client *http.Client) {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
		require.NoError(t, err)
		req.Header.Set("Accept", internal.AcceptHeader)
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// First "invocation": no persisted token, so exactly one token is minted and captured.
	var minted []*oauth2.Token
	client1 := transport.NewServiceAccountClientWithHost(t.Context(), tc.ClientID, tc.ClientSecret, tc.BaseURL, version, nil,
		func(tok *oauth2.Token) error {
			minted = append(minted, tok)
			return nil
		})
	require.NotNil(t, client1)
	doCall(t, client1)

	require.Len(t, minted, 1, "first invocation should mint and persist exactly one token")
	seedToken := minted[0]
	require.NotEmpty(t, seedToken.AccessToken)

	// Subsequent "invocations": seed each new client from the persisted token. No new token should
	// be minted, and the calls must still succeed using the reused token.
	seed := &atlasauth.Token{
		AccessToken: seedToken.AccessToken,
		TokenType:   "Bearer",
		Expiry:      seedToken.Expiry,
	}
	const invocations = 5
	for i := range invocations {
		var newlyMinted []*oauth2.Token
		client := transport.NewServiceAccountClientWithHost(t.Context(), tc.ClientID, tc.ClientSecret, tc.BaseURL, version, seed,
			func(tok *oauth2.Token) error {
				newlyMinted = append(newlyMinted, tok)
				return nil
			})
		require.NotNil(t, client)
		doCall(t, client)
		assert.Empty(t, newlyMinted, "invocation %d should reuse the persisted token, not mint a new one", i+1)
	}

	assert.Len(t, minted, 1, "only the first invocation should have minted a token across %d total calls", invocations+1)
	t.Logf("Service account token reused across %d invocations with a single mint", invocations+1)
}
