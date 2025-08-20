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

package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mongodb/atlas-cli-core/transport"
	"github.com/stretchr/testify/require"
)

const (
	defaultBaseURL          = "https://cloud-dev.mongodb.com/"
	AcceptHeader            = "application/vnd.atlas.2025-03-12+json"
	secretExpiresAfterHours = 8
	contextTimeout          = 30 * time.Second
)

// TestCredentials holds the test credentials and manages lifecycle
type TestCredentials struct {
	t            *testing.T
	OrgID        string
	PublicKey    string
	PrivateKey   string
	ClientID     string
	ClientSecret string
	BaseURL      string
}

func NewTestCredentials(t *testing.T) *TestCredentials {
	t.Helper()

	publicKey := os.Getenv("MONGODB_ATLAS_PUBLIC_API_KEY")
	privateKey := os.Getenv("MONGODB_ATLAS_PRIVATE_API_KEY")
	orgID := os.Getenv("MONGODB_ATLAS_ORG_ID")

	if publicKey == "" || privateKey == "" || orgID == "" {
		t.Skip("E2E tests require MONGODB_ATLAS_PUBLIC_API_KEY, MONGODB_ATLAS_PRIVATE_API_KEY, and MONGODB_ATLAS_ORG_ID")
	}

	baseURL := defaultBaseURL
	if url := os.Getenv("MONGODB_ATLAS_OPS_MANAGER_URL"); url != "" {
		baseURL = url
	}

	return &TestCredentials{
		t:          t,
		OrgID:      orgID,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		BaseURL:    baseURL,
	}
}

// serviceAccountResponse represents the API response when creating a service account
type serviceAccountResponse struct {
	ClientID string `json:"clientId"`
	Secrets  []struct {
		Secret string `json:"secret"`
	} `json:"secrets"`
}

// CreateServiceAccount provisions a service account and sets the client ID and secret in the TestCredentials.
func (tc *TestCredentials) CreateServiceAccount() {
	tc.t.Helper()

	if tc.ClientID != "" && tc.ClientSecret != "" {
		tc.t.Fatal("service account already exists")
	}

	// Create service account using digest auth
	digestTransport := transport.NewDigestTransport(tc.PublicKey, tc.PrivateKey, transport.Default())
	client := &http.Client{Transport: digestTransport}

	payload := map[string]any{
		"description":             "Test service account for atlas-cli-core",
		"name":                    fmt.Sprintf("transport-e2e-test-%d", time.Now().Unix()),
		"roles":                   []string{"ORG_OWNER"},
		"secretExpiresAfterHours": secretExpiresAfterHours,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(tc.t, err)

	// Create the service account
	url := fmt.Sprintf("%sapi/atlas/v2/orgs/%s/serviceAccounts", tc.BaseURL, tc.OrgID)
	req, err := http.NewRequestWithContext(tc.t.Context(), http.MethodPost, url, bytes.NewReader(payloadBytes))
	require.NoError(tc.t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", AcceptHeader)

	resp, err := client.Do(req)
	require.NoError(tc.t, err)
	defer resp.Body.Close()

	require.Less(tc.t, resp.StatusCode, http.StatusBadRequest, "Failed to create service account, status: %d", resp.StatusCode)

	// Setup cleanup
	tc.t.Cleanup(func() {
		tc.t.Logf("Cleaning up service account: %s", tc.ClientID)
		tc.deleteServiceAccount()
	})

	// Extract service account credentials
	var serviceAccount serviceAccountResponse
	err = json.NewDecoder(resp.Body).Decode(&serviceAccount)
	require.NoError(tc.t, err)

	tc.ClientID = serviceAccount.ClientID
	require.NotEmpty(tc.t, tc.ClientID, "clientID should not be empty")

	require.NotEmpty(tc.t, serviceAccount.Secrets, "secrets should not be empty")
	tc.ClientSecret = serviceAccount.Secrets[0].Secret
	require.NotEmpty(tc.t, tc.ClientSecret, "clientSecret should not be empty")

	tc.t.Logf("Created service account: clientID=%s", tc.ClientID)
}

// deleteServiceAccount deletes the service account created for testing.
func (tc *TestCredentials) deleteServiceAccount() {
	tc.t.Helper()

	if tc.ClientID == "" {
		return
	}

	// Create a context with timeout for the delete request as test context is cancelled
	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	digestTransport := transport.NewDigestTransport(tc.PublicKey, tc.PrivateKey, transport.Default())
	client := &http.Client{Transport: digestTransport}

	url := fmt.Sprintf("%sapi/atlas/v2/orgs/%s/serviceAccounts/%s", tc.BaseURL, tc.OrgID, tc.ClientID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		tc.t.Logf("Failed to create delete request: %v", err)
		return
	}
	req.Header.Set("Accept", AcceptHeader)

	resp, err := client.Do(req)
	if err != nil {
		tc.t.Logf("Failed to delete service account %s: %v", tc.ClientID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		tc.t.Logf("Delete request failed with status %d for service account %s", resp.StatusCode, tc.ClientID)
	}
}
