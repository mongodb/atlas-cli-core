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
	"os"
	"testing"

	"github.com/mongodb/atlas-cli-core/config"
	"github.com/mongodb/atlas-cli-core/test/internal"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAtlasCLICoreLibrary tests the most essential library functionality that plugin developers use.
// This focuses on the core workflow without trying to test every edge case.
func TestAtlasCLICoreLibrary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	tempDir := t.TempDir()

	// Set environment variable to override config directory
	// This must be done before any config operations
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Clear and reconfigure viper for isolated test
	viper.Reset()

	// Test the essential plugin developer workflow
	profileName := "test-plugin-profile"

	// Initialize profile (what plugin developers do first)
	err := config.InitProfile(profileName)
	require.NoError(t, err)
	assert.Equal(t, profileName, config.Name())

	// Set basic configuration (core plugin operations)
	config.SetService(config.CloudService)

	// Verify configuration can be read
	assert.Equal(t, config.CloudService, config.Service())
	assert.True(t, config.IsCloud())
	assert.Equal(t, os.Getenv("MONGODB_ATLAS_ORG_ID"), config.OrgID())
	assert.Equal(t, os.Getenv("MONGODB_ATLAS_PROJECT_ID"), config.ProjectID())

	// Test profile existence checks
	assert.True(t, config.Exists(profileName))
	assert.False(t, config.Exists("nonexistent-profile"))

	// Test profile listing
	profiles := config.List()
	assert.Contains(t, profiles, profileName)
}

func TestProfileInitializationWithEnvironmentVariables(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	t.Run("APIKeysFromEnvironment", func(t *testing.T) {
		internal.TempConfigFolder(t)
		viper.Reset()

		t.Setenv("MONGODB_ATLAS_PUBLIC_API_KEY", "test_public_key")
		t.Setenv("MONGODB_ATLAS_PRIVATE_API_KEY", "test_private_key")
		t.Setenv("MONGODB_ATLAS_PROJECT_ID", "test_project_id")
		t.Setenv("MONGODB_ATLAS_ORG_ID", "test_org_id")

		profile, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		err = config.InitProfile("")
		require.NoError(t, err)

		assert.Equal(t, "test_public_key", config.PublicAPIKey())
		assert.Equal(t, "test_private_key", config.PrivateAPIKey())
		assert.Equal(t, "test_project_id", config.ProjectID())
		assert.Equal(t, "test_org_id", config.OrgID())
		assert.Equal(t, config.APIKeys, config.AuthType())
		assert.Equal(t, "default", profile.Name())
	})

	t.Run("ServiceAccountFromEnvironment", func(t *testing.T) {
		internal.TempConfigFolder(t)
		viper.Reset()

		t.Setenv("MONGODB_ATLAS_CLIENT_ID", "test_client_id")
		t.Setenv("MONGODB_ATLAS_CLIENT_SECRET", "test_client_secret")
		t.Setenv("MONGODB_ATLAS_PROJECT_ID", "test_project_id")

		profile, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		err = config.InitProfile("")
		require.NoError(t, err)

		assert.Equal(t, "test_client_id", config.ClientID())
		assert.Equal(t, "test_client_secret", config.ClientSecret())
		assert.Equal(t, "test_project_id", config.ProjectID())
		assert.Equal(t, config.ServiceAccount, config.AuthType())
		assert.Equal(t, "default", profile.Name())
	})

	t.Run("ProfileSelectionFromEnvironment", func(t *testing.T) {
		internal.TempConfigFolder(t)
		viper.Reset()

		t.Setenv("MONGODB_ATLAS_PROFILE", "env-profile")
		t.Setenv("MONGODB_ATLAS_PROJECT_ID", "env_project")

		_, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		err = config.InitProfile("")
		require.NoError(t, err)

		assert.Equal(t, "env-profile", config.Name())
		assert.Equal(t, "env_project", config.ProjectID())
	})

	t.Run("LegacyMCLIPrefix", func(t *testing.T) {
		internal.TempConfigFolder(t)
		viper.Reset()

		t.Setenv("MCLI_PUBLIC_API_KEY", "legacy_public")
		t.Setenv("MCLI_PRIVATE_API_KEY", "legacy_private")
		t.Setenv("MCLI_ORG_ID", "legacy_org")

		_, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		err = config.InitProfile("")
		require.NoError(t, err)

		assert.Equal(t, "legacy_public", config.PublicAPIKey())
		assert.Equal(t, "legacy_private", config.PrivateAPIKey())
		assert.Equal(t, "legacy_org", config.OrgID())
		assert.Equal(t, config.APIKeys, config.AuthType())
	})
}
