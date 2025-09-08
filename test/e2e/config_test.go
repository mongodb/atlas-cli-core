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
	"path"
	"path/filepath"
	"testing"

	"github.com/mongodb/atlas-cli-core/config"
	"github.com/mongodb/atlas-cli-core/test/internal"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig tests the complete config loading flow in a real environment
func TestConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	t.Run("LoadAtlasCLIConfig with empty config succeeds", func(t *testing.T) {
		dir := internal.TempConfigFolder(t)
		configPath := path.Join(dir, "config.toml")
		err := os.WriteFile(configPath, []byte(""), 0600)
		require.NoError(t, err)

		profile, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, "default", profile.Name())
	})

	t.Run("LoadAtlasCLIConfig with version 1 config fails", func(t *testing.T) {
		withOldConfig(t)
		profile, err := config.LoadAtlasCLIConfig()
		require.Nil(t, profile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "config version is missing")
	})

	t.Run("LoadAtlasCLIConfig with version 2 succeeds", func(t *testing.T) {
		withNewConfig(t)
		profile, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, "default", profile.Name())

		// check that it has not set any profile properties yet
		assert.Empty(t, profile.Service())

		// init profile (this is what plugin developers do after loading the config)
		err = config.InitProfile("config_test")
		require.NoError(t, err)

		assert.Equal(t, "cloud", profile.Service())
		assert.Equal(t, "config_test", profile.Name())
	})

	t.Run("LoadAtlasCLIConfigWithVersion(2) succeeds", func(t *testing.T) {
		withNewConfig(t)
		profile, err := config.LoadAtlasCLIConfigWithVersion(2)
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, "default", profile.Name())
	})
}

// TestConfigWithEnvironmentVariables tests that we can load and initialize config from environment variables.
func TestConfigWithEnvironmentVariables(t *testing.T) {
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

// TestMixedConfigurationSources tests that we can load and initialize config from environment variables and file.
func TestMixedConfigurationSources(t *testing.T) {
	t.Run("EnvironmentOverridesFile", func(t *testing.T) {
		dir := internal.TempConfigFolder(t)
		viper.Reset()

		// Create config file with values
		configPath := filepath.Join(dir, "config.toml")
		configContent := `version = 2
[default]
project_id = "file_project"
org_id = "file_org"
client_id = "file_client_id"
client_secret = "file_client_secret"
service = "cloud"
`
		err := os.WriteFile(configPath, []byte(configContent), 0600)
		require.NoError(t, err)

		// Set environment variables that should override file
		t.Setenv("MONGODB_ATLAS_PROJECT_ID", "env_project")
		t.Setenv("MONGODB_ATLAS_ORG_ID", "env_org")

		_, err = config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		err = config.InitProfile("default")
		require.NoError(t, err)

		// Environment should override file
		assert.Equal(t, "env_project", config.ProjectID())
		assert.Equal(t, "env_org", config.OrgID())
		// File values should be used where no env override
		assert.Equal(t, "file_client_id", config.ClientID())
		assert.Equal(t, "file_client_secret", config.ClientSecret())
	})

	t.Run("MCLIFallbackWhenNoAtlasEnv", func(t *testing.T) {
		internal.TempConfigFolder(t)
		viper.Reset()

		// Set only MCLI vars when MONGODB_ATLAS vars are not present
		t.Setenv("MCLI_PROJECT_ID", "mcli_project")
		t.Setenv("MCLI_ORG_ID", "mcli_org")

		_, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		err = config.InitProfile("")
		require.NoError(t, err)

		assert.Equal(t, "mcli_project", config.ProjectID())
		assert.Equal(t, "mcli_org", config.OrgID())
	})

	t.Run("ProfileSpecificVsGlobalProperties", func(t *testing.T) {
		dir := internal.TempConfigFolder(t)
		viper.Reset()

		// Create config with global and profile-specific settings
		configPath := filepath.Join(dir, "config.toml")
		configContent := `version = 2
skip_update_check = true
telemetry_enabled = false

[test-profile]
project_id = "profile_project"
service = "cloud"
`
		err := os.WriteFile(configPath, []byte(configContent), 0600)
		require.NoError(t, err)

		_, err = config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		err = config.InitProfile("test-profile")
		require.NoError(t, err)

		assert.Equal(t, "test-profile", config.Name())
		assert.Equal(t, "profile_project", config.ProjectID())
		assert.True(t, config.SkipUpdateCheck())
		assert.False(t, config.TelemetryEnabled())
	})
}

// loads an old config with version 1
func withOldConfig(t *testing.T) {
	t.Helper()
	dir := internal.TempConfigFolder(t)

	configPath := path.Join(dir, "config.toml")

	err := os.WriteFile(configPath, []byte(`[config_test]
  org_id = "test_id"
  public_api_key = "test_pub"
  service = "cloud"
`), 0600)
	require.NoError(t, err)
}

// loads a new config with version 2
func withNewConfig(t *testing.T) {
	t.Helper()
	dir := internal.TempConfigFolder(t)

	configPath := path.Join(dir, "config.toml")

	err := os.WriteFile(configPath, []byte(`
  silence_storage_warning = true
  version = 2
  [config_test]
  org_id = "new_config_org_id"
  client_id = "new_config_client_id"
  client_secret = "new_config_client_secret"
  service = "cloud"
`), 0600)
	require.NoError(t, err)
}
