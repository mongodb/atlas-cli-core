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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigLoadingE2E tests the complete config loading flow in a real environment
func TestConfigLoadingE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	t.Run("LoadAtlasCLIConfig functionality", func(t *testing.T) {
		// Test that the function is callable and returns appropriate errors or profiles
		profile, err := config.LoadAtlasCLIConfig()

		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, config.DefaultProfile, profile.Name())
		t.Logf("Config loaded successfully with profile: %s", profile.Name())

	})

	t.Run("LoadAtlasCLIConfigWithVersion functionality", func(t *testing.T) {
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
}

// TestProfileLifecycleE2E tests complete profile management operations
func TestProfileLifecycleE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// Use a clean in-memory store for this test to avoid environment pollution
	store := config.NewInMemoryStore()
	profile := config.NewProfile("e2e-test-profile", store)

	// Store original default profile and restore it later
	originalProfile := config.Default()
	defer config.SetDefaultProfile(originalProfile)

	t.Run("profile creation and configuration", func(t *testing.T) {
		// Set the test profile as default
		config.SetDefaultProfile(profile)

		// Configure the profile
		profile.SetProjectID("64f1a5b2c3d4e5f6789012ab")
		profile.SetOrgID("64f1a5b2c3d4e5f6789012cd")
		profile.SetOutput("json")
		profile.SetService(config.CloudService)

		// Verify values are set
		assert.Equal(t, "64f1a5b2c3d4e5f6789012ab", profile.ProjectID())
		assert.Equal(t, "64f1a5b2c3d4e5f6789012cd", profile.OrgID())
		assert.Equal(t, "json", profile.Output())
		assert.Equal(t, config.CloudService, profile.Service())

		// Test cloud service detection
		assert.True(t, profile.Service() == config.CloudService || profile.Service() == "" || profile.Service() == config.CloudGovService)
	})

	t.Run("profile validation", func(t *testing.T) {
		// Test profile name validation using the profile directly
		err := profile.SetName("invalid.profile.name")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "profile should not contain '.'")

		// Test valid profile names
		validNames := []string{"valid-profile", "profile123", "test_profile"}
		for _, name := range validNames {
			err := profile.SetName(name)
			assert.NoError(t, err, "profile name %s should be valid", name)
		}
	})

	t.Run("profile configuration mapping", func(t *testing.T) {
		// Set the test profile as default
		config.SetDefaultProfile(profile)

		// Test configuration mapping and redaction
		configMap := profile.Map()
		require.NotNil(t, configMap) // May be empty but should not be nil

		// Test sorted keys
		sortedKeys := profile.SortedKeys()
		require.NotNil(t, sortedKeys) // May be empty but should not be nil

		// Verify keys are actually sorted if there are any
		for i := 1; i < len(sortedKeys); i++ {
			assert.LessOrEqual(t, sortedKeys[i-1], sortedKeys[i], "keys should be sorted")
		}
	})
}

// TestAuthenticationConfigE2E tests authentication-related configuration
func TestAuthenticationConfigE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// Use a clean in-memory store for this test
	store := config.NewInMemoryStore()
	profile := config.NewProfile("auth-test-profile", store)

	// Store original default profile and restore it later
	originalProfile := config.Default()
	defer config.SetDefaultProfile(originalProfile)

	// Set the test profile as default
	config.SetDefaultProfile(profile)

	t.Run("API key authentication", func(t *testing.T) {
		// Set API key credentials using the profile directly
		profile.SetPublicAPIKey("test-public-key")
		profile.SetPrivateAPIKey("test-private-key")

		// Verify values are set
		assert.Equal(t, "test-public-key", profile.PublicAPIKey())
		assert.Equal(t, "test-private-key", profile.PrivateAPIKey())

		// Test access validation
		assert.True(t, profile.IsAccessSet())

		// Test manual auth type setting
		profile.SetAuthType(config.APIKeys)
		assert.Equal(t, config.APIKeys, profile.AuthType())
	})

	t.Run("service account authentication", func(t *testing.T) {
		// Clear previous auth settings
		profile.SetPublicAPIKey("")
		profile.SetPrivateAPIKey("")

		// Set service account credentials
		profile.SetClientID("test-client-id")
		profile.SetClientSecret("test-client-secret")

		// Verify values are set
		assert.Equal(t, "test-client-id", profile.ClientID())
		assert.Equal(t, "test-client-secret", profile.ClientSecret())

		// Test access validation
		assert.True(t, profile.IsAccessSet())

		// Test manual auth type setting
		profile.SetAuthType(config.ServiceAccount)
		assert.Equal(t, config.ServiceAccount, profile.AuthType())
	})

	t.Run("oauth authentication", func(t *testing.T) {
		// Clear previous auth settings
		profile.SetClientID("")
		profile.SetClientSecret("")

		// Set OAuth tokens
		profile.SetAccessToken("test-access-token")
		profile.SetRefreshToken("test-refresh-token")

		// Verify values are set
		assert.Equal(t, "test-access-token", profile.AccessToken())
		assert.Equal(t, "test-refresh-token", profile.RefreshToken())

		// Test manual auth type setting
		profile.SetAuthType(config.UserAccount)
		assert.Equal(t, config.UserAccount, profile.AuthType())
	})

	t.Run("auth type explicit setting", func(t *testing.T) {
		// Test explicit auth type setting using the profile
		profile.SetAuthType(config.NoAuth)
		assert.Equal(t, config.NoAuth, profile.AuthType())

		profile.SetAuthType(config.APIKeys)
		assert.Equal(t, config.APIKeys, profile.AuthType())
	})
}

// TestConfigGlobalSettingsE2E tests global configuration settings
func TestConfigGlobalSettingsE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	// Use a clean in-memory store for this test
	store := config.NewInMemoryStore()
	profile := config.NewProfile("global-test-profile", store)

	// Store original default profile and restore it later
	originalProfile := config.Default()
	defer config.SetDefaultProfile(originalProfile)

	// Set the test profile as default
	config.SetDefaultProfile(profile)

	t.Run("global settings management", func(t *testing.T) {
		// Test skip update check
		config.SetSkipUpdateCheck(true)
		assert.True(t, config.SkipUpdateCheck())

		config.SetSkipUpdateCheck(false)
		assert.False(t, config.SkipUpdateCheck())

		// Test telemetry settings
		if !config.IsTelemetryEnabledSet() {
			config.SetTelemetryEnabled(true)
			assert.True(t, config.TelemetryEnabled())
			assert.True(t, config.IsTelemetryEnabledSet())

			config.SetTelemetryEnabled(false)
			assert.False(t, config.TelemetryEnabled())
		}

		// Test local deployment image setting
		testImage := "mongo:7.0"
		config.SetLocalDeploymentImage(testImage)
		assert.Equal(t, testImage, config.GetLocalDeploymentImage())
	})

	t.Run("service configuration", func(t *testing.T) {
		// Test cloud service (default)
		config.SetService(config.CloudService)
		assert.Equal(t, config.CloudService, config.Service())
		assert.True(t, config.IsCloud())

		// Test cloud gov service
		config.SetService(config.CloudGovService)
		assert.Equal(t, config.CloudGovService, config.Service())
		assert.True(t, config.IsCloud())

		// Test other service (should not be cloud)
		config.SetService("other")
		assert.Equal(t, "other", config.Service())
		assert.False(t, config.IsCloud())
	})
}

func TestUnsecureStorageWarnsAndSucceeds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	store := config.NewInMemoryStore()
	profile := config.NewProfile("unsecure-storage-test-profile", store)

}
