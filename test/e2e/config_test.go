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
	"fmt"
	"testing"

	"github.com/mongodb/atlas-cli-core/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigLoadingE2E tests the complete config loading flow in a real environment
func TestConfigLoadingE2E(t *testing.T) {
	tempDir := t.TempDir()

	// Set environment variable to override config directory
	// This must be done before any config operations
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Clear and reconfigure viper for isolated test
	viper.Reset()

	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	t.Run("LoadAtlasCLIConfig", func(t *testing.T) {
		// Test that the function is callable and returns appropriate errors or profiles
		profile, err := config.LoadAtlasCLIConfig()

		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, config.DefaultProfile, profile.Name())
		t.Logf("Config loaded successfully with profile: %s", profile.Name())
	})

	t.Run("LoadAtlasCLIConfigWithVersion", func(t *testing.T) {
		// Test different version scenarios
		testData := []struct {
			version int64
			wantErr bool
		}{
			{version: 1, wantErr: true},
			{version: 2, wantErr: false}, // expected version is version 2
			{version: 3, wantErr: true},
		}

		for _, tt := range testData {
			t.Run(fmt.Sprintf("version_%d", tt.version), func(t *testing.T) {
				profile, err := config.LoadAtlasCLIConfigWithVersion(tt.version)

				if tt.wantErr {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					require.NotNil(t, profile)
					assert.Equal(t, config.DefaultProfile, profile.Name())
					t.Logf("Config loaded successfully for version %d", tt.version)
				}
			})
		}
	})

	t.Run("Can get all profiles with DefaultStore", func(t *testing.T) {
		store, err := config.NewDefaultStore()
		require.NoError(t, err)
		require.NotNil(t, store)

		profiles := store.GetProfileNames()
		assert.NotEmpty(t, profiles)

		// check that the secure store is available
		assert.True(t, store.IsSecure())
	})
}
