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
	"net/http"
	"testing"

	"github.com/mongodb/atlas-cli-core/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAuthIssuerGetter struct {
	service       string
	clientID      string
	authServerURL string
}

func (f *fakeAuthIssuerGetter) Service() string       { return f.service }
func (f *fakeAuthIssuerGetter) ClientID() string      { return f.clientID }
func (f *fakeAuthIssuerGetter) AuthServerURL() string { return f.authServerURL }

func TestFlowForAuthIssuer(t *testing.T) {
	tests := []struct {
		name              string
		getter            *fakeAuthIssuerGetter
		expectedClientID  string
		expectedAuthURL   string
		expectedErrSubstr string
	}{
		{
			name:             "commercial defaults",
			getter:           &fakeAuthIssuerGetter{service: config.CloudService},
			expectedClientID: authIssuerClientID,
			expectedAuthURL:  "https://authorize.mongodb.com",
		},
		{
			name:             "gov defaults",
			getter:           &fakeAuthIssuerGetter{service: config.CloudGovService},
			expectedClientID: govAuthIssuerClientID,
			expectedAuthURL:  govDefaultAuthIssuerURL,
		},
		{
			name:             "client ID override",
			getter:           &fakeAuthIssuerGetter{service: config.CloudService, clientID: "override-id"},
			expectedClientID: "override-id",
			expectedAuthURL:  "https://authorize.mongodb.com",
		},
		{
			name:             "auth server URL override beats gov default",
			getter:           &fakeAuthIssuerGetter{service: config.CloudGovService, authServerURL: "https://override.example.com"},
			expectedClientID: govAuthIssuerClientID,
			expectedAuthURL:  "https://override.example.com",
		},
		{
			name:              "invalid auth server URL",
			getter:            &fakeAuthIssuerGetter{service: config.CloudService, authServerURL: "://bad"},
			expectedErrSubstr: "missing protocol scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := FlowForAuthIssuer(tt.getter, http.DefaultClient, testVersion)
			if tt.expectedErrSubstr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrSubstr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedClientID, cfg.ClientID)
			assert.Equal(t, tt.expectedAuthURL, cfg.AuthServerURL.String())
			assert.Equal(t, []string{"atlas"}, cfg.Scopes)
		})
	}
}
