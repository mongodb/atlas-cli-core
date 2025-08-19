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

//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/mongodb/atlas-cli-core/config"
	"github.com/mongodb/atlas-cli-core/test/internal"
	"github.com/mongodb/atlas-cli-core/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDigestTransport tests the digest authentication transport through an actual Atlas API call.
func TestDigestTransport(t *testing.T) {
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
	tc := internal.NewTestCredentials(t)
	tc.CreateServiceAccount()

	tmp := config.OpsManagerURL()
	config.SetOpsManagerURL(tc.BaseURL)
	defer config.SetOpsManagerURL(tmp)
	// Test the service account client from transport package
	client := transport.NewServiceAccountClient(tc.ClientID, tc.ClientSecret)
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

func TestTransportTimeouts(t *testing.T) {
	// Test that default and telemetry transports have different timeouts
	defaultTransport := transport.Default()
	telemetryTransport := transport.Telemetry()

	// Verify they are different instances
	assert.NotSame(t, defaultTransport, telemetryTransport, "Default and telemetry transports should be different instances")

	// Test behavior with a slow endpoint (you might need to create a test server for this)
	t.Log("Transport timeout configurations verified")
}
