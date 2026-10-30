// Copyright 2026 MongoDB Inc
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
	"os"
	"path/filepath"
	"testing"

	"github.com/mongodb/atlas-cli-core/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
	"go.mongodb.org/atlas-sdk/v20250312027/auth/code"
)

// TestFlowForAuthServerIssuer verifies issuer precedence through a real store and
// real environment variables, including the legacy MCLI_* env prefix.
func TestFlowForAuthServerIssuer(t *testing.T) {
	tests := []struct {
		name     string
		profile  string
		env      string
		mcliEnv  bool
		expected string
		wantErr  bool
	}{
		{name: "production default", expected: code.DefaultAuthServerURL},
		{name: "gov default", profile: "service = \"cloudgov\"\n", expected: govAuthServerURL},
		{name: "config only", profile: "auth_server_url = \"https://config.example.com\"\n", expected: "https://config.example.com"},
		{name: "env only", env: "https://env.example.com", expected: "https://env.example.com"},
		{
			name:     "env overrides config",
			profile:  "auth_server_url = \"https://config.example.com\"\n",
			env:      "https://env.example.com",
			expected: "https://env.example.com",
		},
		{
			name:     "config overrides gov default",
			profile:  "service = \"cloudgov\"\nauth_server_url = \"https://config.example.com\"\n",
			expected: "https://config.example.com",
		},
		{
			name:     "stored issuer wins over env and config",
			profile:  "auth_server_issuer = \"https://stored.example.com\"\nauth_server_url = \"https://config.example.com\"\n",
			env:      "https://env.example.com",
			expected: "https://stored.example.com",
		},
		{
			name:     "env overrides config with MCLI variables set",
			profile:  "auth_server_url = \"https://config.example.com\"\n",
			env:      "https://env.example.com",
			mcliEnv:  true,
			expected: "https://env.example.com",
		},
		{name: "non-HTTPS config rejected", profile: "auth_server_url = \"http://config.example.com\"\n", wantErr: true},
		{name: "non-HTTPS env rejected", env: "http://env.example.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", home)
			dir, err := os.UserConfigDir()
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(filepath.Join(dir, config.AtlasCLI), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, config.AtlasCLI, "config.toml"),
				[]byte("version = 3\n[default]\n"+tt.profile), 0o600))
			keyring.MockInit()
			t.Setenv("MONGODB_ATLAS_AUTH_SERVER_URL", tt.env)
			if tt.mcliEnv {
				t.Setenv("MCLI_ORG_ID", "legacy_org")
			}

			store, err := config.NewDefaultStore()
			require.NoError(t, err)

			flow, err := FlowForAuthServer(config.NewProfile(config.DefaultProfile, store), nil, testVersion)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, flow.AuthServerURL.String())
		})
	}
}

func TestFlowForAuthServerClientID(t *testing.T) {
	tests := []struct {
		name     string
		authEnv  string
		saEnv    string
		expected string
	}{
		{name: "default", expected: authCodeClientID},
		{name: "MONGODB_ATLAS_AUTH_CLIENT_ID overrides", authEnv: "custom-client", expected: "custom-client"},
		{name: "service-account MONGODB_ATLAS_CLIENT_ID is ignored", saEnv: "sa-client", expected: authCodeClientID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MONGODB_ATLAS_AUTH_CLIENT_ID", tt.authEnv)
			t.Setenv("MONGODB_ATLAS_CLIENT_ID", tt.saEnv)

			flow, err := FlowForAuthServer(config.NewProfile(config.DefaultProfile, config.NewInMemoryStore()), nil, testVersion)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, flow.ClientID)
		})
	}
}
