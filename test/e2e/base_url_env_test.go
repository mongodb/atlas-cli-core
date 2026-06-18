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
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mongodb/atlas-cli-core/config"
	"github.com/mongodb/atlas-cli-core/transport"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBaseURLEnvVarDrivesRequests is the integration regression test for
// CLOUDP-383021. It wires the real config store, profile, and transport client
// together to prove that MONGODB_ATLAS_BASE_URL actually steers outgoing
// requests, not just that it resolves in the config layer.
//
// Service account auth is used because HTTPClientFromProfile feeds
// OpsManagerURL() straight into the OAuth token URL, so a request to the client
// only reaches our test server when the env var is honored. Before the fix
// OpsManagerURL() was empty and the token request went to the production
// default instead.
func TestBaseURLEnvVarDrivesRequests(t *testing.T) {
	var tokenRequests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/oauth/token" {
			tokenRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"fake-token","token_type":"bearer","expires_in":3600}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Clear any inherited ops_manager_url so base_url is the only source for the
	// URL. The e2e CI environment sets MONGODB_ATLAS_OPS_MANAGER_URL for the
	// credential-based tests, and it correctly takes precedence over base_url.
	t.Setenv("MONGODB_ATLAS_OPS_MANAGER_URL", "")
	// auth_type is set explicitly so the service account branch is taken
	// without depending on AuthType()'s credential inference.
	t.Setenv("MONGODB_ATLAS_AUTH_TYPE", string(config.ServiceAccount))
	t.Setenv("MONGODB_ATLAS_CLIENT_ID", "test-client-id")
	t.Setenv("MONGODB_ATLAS_CLIENT_SECRET", "test-client-secret")
	t.Setenv("MONGODB_ATLAS_BASE_URL", server.URL)

	store, err := config.NewViperStore(afero.NewMemMapFs(), true)
	require.NoError(t, err)
	profile := config.NewProfile(config.DefaultProfile, store)

	require.Equal(t, config.ServiceAccount, profile.AuthType())
	require.Equal(t, server.URL, profile.OpsManagerURL())

	client, err := transport.HTTPClientFromProfile(profile, "test", transport.Default())
	require.NoError(t, err)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/api/atlas/v2/orgs", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Positive(t, tokenRequests.Load(),
		"OAuth token request should have reached the MONGODB_ATLAS_BASE_URL host")
}
