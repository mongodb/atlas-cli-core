// Copyright 2024 MongoDB Inc
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

package config

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// CLOUDP-383021: MONGODB_ATLAS_BASE_URL must resolve the base URL. base_url is a
// viper alias of ops_manager_url, but aliases only apply to the config file, so
// without an explicit env binding the variable was silently ignored.
func TestViperStore_BaseURLEnvVar(t *testing.T) {
	tests := []struct {
		name           string
		opsManagerEnv  string
		baseURLEnv     string
		wantOpsManager string
	}{
		{
			name:           "base url env var is honored",
			baseURLEnv:     "https://cloud-dev.mongodb.com/",
			wantOpsManager: "https://cloud-dev.mongodb.com/",
		},
		{
			name:           "ops manager url env var still works",
			opsManagerEnv:  "https://cloud-qa.mongodb.com/",
			wantOpsManager: "https://cloud-qa.mongodb.com/",
		},
		{
			name:           "ops manager url takes precedence when both are set",
			opsManagerEnv:  "https://cloud-qa.mongodb.com/",
			baseURLEnv:     "https://cloud-dev.mongodb.com/",
			wantOpsManager: "https://cloud-qa.mongodb.com/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opsManagerEnv != "" {
				t.Setenv(AtlasCLIEnvPrefix+"_OPS_MANAGER_URL", tt.opsManagerEnv)
			}
			if tt.baseURLEnv != "" {
				t.Setenv(AtlasCLIEnvPrefix+"_BASE_URL", tt.baseURLEnv)
			}

			store, err := NewViperStore(afero.NewMemMapFs(), true)
			require.NoError(t, err)

			profile := NewProfile(DefaultProfile, store)
			require.Equal(t, tt.wantOpsManager, profile.OpsManagerURL())
		})
	}
}
