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
	"path/filepath"
	"testing"

	"github.com/mongodb/atlas-cli-core/config"
	"github.com/mongodb/atlas-cli-core/test/internal"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitProfile tests the most essential library functionality that plugin developers use.
// This focuses on the core workflow without trying to test every edge case.
func TestInitProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	_ = internal.TempConfigFolder(t)
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

func TestProfileValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	t.Run("UnsupportedServiceType", func(t *testing.T) {
		dir := internal.TempConfigFolder(t)
		viper.Reset()

		configPath := filepath.Join(dir, "config.toml")
		configContent := `version = 2

[invalid-service]
project_id = "test_project"
service = "unsupported"
`
		err := os.WriteFile(configPath, []byte(configContent), 0600)
		require.NoError(t, err)

		_, err = config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		err = config.InitProfile("invalid-service")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported service")
	})

	t.Run("ValidCloudServices", func(t *testing.T) {
		dir := internal.TempConfigFolder(t)
		viper.Reset()

		configPath := filepath.Join(dir, "config.toml")
		configContent := `version = 2

[cloud-profile]
project_id = "test_project"
service = "cloud"

[cloudgov-profile]
project_id = "test_project"
service = "cloudgov"

[default-profile]
project_id = "test_project"
# no service specified - should default to cloud
`
		err := os.WriteFile(configPath, []byte(configContent), 0600)
		require.NoError(t, err)

		_, err = config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		// Test cloud service
		err = config.InitProfile("cloud-profile")
		require.NoError(t, err)
		assert.True(t, config.IsCloud())
		assert.Equal(t, "cloud", config.Service())

		// Test cloudgov service
		err = config.SetName("cloudgov-profile")
		require.NoError(t, err)
		assert.True(t, config.IsCloud())
		assert.Equal(t, "cloudgov", config.Service())

		// Test default (empty) service
		err = config.SetName("default-profile")
		require.NoError(t, err)
		assert.True(t, config.IsCloud())
		assert.Empty(t, config.Service())
	})

	t.Run("ProfileNameCaseInsensitive", func(t *testing.T) {
		dir := internal.TempConfigFolder(t)
		viper.Reset()

		configPath := filepath.Join(dir, "config.toml")
		configContent := `version = 2

[test-profile]
project_id = "test_project"
service = "cloud"
`
		err := os.WriteFile(configPath, []byte(configContent), 0600)
		require.NoError(t, err)

		_, err = config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		// Profile names should be case-insensitive
		err = config.InitProfile("TEST-PROFILE")
		require.NoError(t, err)
		assert.Equal(t, "test-profile", config.Name())

		err = config.SetName("Test-Profile")
		require.NoError(t, err)
		assert.Equal(t, "test-profile", config.Name())
	})

	t.Run("NonexistentProfileHandling", func(t *testing.T) {
		internal.TempConfigFolder(t)
		viper.Reset()

		_, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		// Initialize with non-existent profile name should succeed
		err = config.InitProfile("new-profile")
		require.NoError(t, err)
		assert.Equal(t, "new-profile", config.Name())

		// Profile doesn't exist in the store until we set some values
		config.Set("project_id", "test_project")
		config.Set("service", "cloud")

		// Now it should exist
		assert.True(t, config.Exists("new-profile"))
	})
}

func TestMultipleProfilesE2E(t *testing.T) {
	t.Run("ProfileCreationAndListing", func(t *testing.T) {
		dir := internal.TempConfigFolder(t)
		viper.Reset()

		// Create config with multiple profiles
		configPath := filepath.Join(dir, "config.toml")
		configContent := `version = 2

[dev]
project_id = "dev_project"
service = "cloud"

[prod]
project_id = "prod_project"
service = "cloud"

[staging]
project_id = "staging_project"
service = "cloud"
`
		err := os.WriteFile(configPath, []byte(configContent), 0600)
		require.NoError(t, err)

		_, err = config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		profiles := config.List()
		assert.Contains(t, profiles, "dev")
		assert.Contains(t, profiles, "prod")
		assert.Contains(t, profiles, "staging")
		assert.Len(t, profiles, 3)

		// Test profile existence
		assert.True(t, config.Exists("dev"))
		assert.True(t, config.Exists("prod"))
		assert.False(t, config.Exists("nonexistent"))
	})

	t.Run("ProfileSwitching", func(t *testing.T) {
		dir := internal.TempConfigFolder(t)
		viper.Reset()

		configPath := filepath.Join(dir, "config.toml")
		configContent := `version = 2

[dev]
project_id = "dev_project"
org_id = "dev_org"
service = "cloud"

[prod]
project_id = "prod_project"
org_id = "prod_org"
service = "cloud"
`
		err := os.WriteFile(configPath, []byte(configContent), 0600)
		require.NoError(t, err)

		_, err = config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		// Switch to dev profile
		err = config.InitProfile("dev")
		require.NoError(t, err)
		assert.Equal(t, "dev", config.Name())
		assert.Equal(t, "dev_project", config.ProjectID())
		assert.Equal(t, "dev_org", config.OrgID())

		// Switch to prod profile
		err = config.SetName("prod")
		require.NoError(t, err)
		assert.Equal(t, "prod", config.Name())
		assert.Equal(t, "prod_project", config.ProjectID())
		assert.Equal(t, "prod_org", config.OrgID())
	})

	t.Run("SingleProfileAutoSelection", func(t *testing.T) {
		dir := internal.TempConfigFolder(t)
		viper.Reset()

		configPath := filepath.Join(dir, "config.toml")
		configContent := `version = 2

[single-profile]
project_id = "single_project"
service = "cloud"
`
		err := os.WriteFile(configPath, []byte(configContent), 0600)
		require.NoError(t, err)

		_, err = config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		// Should auto-select the single available profile
		err = config.InitProfile("")
		require.NoError(t, err)
		assert.Equal(t, "single-profile", config.Name())
		assert.Equal(t, "single_project", config.ProjectID())
	})

	t.Run("ProfileRenaming", func(t *testing.T) {
		dir := internal.TempConfigFolder(t)
		viper.Reset()

		configPath := filepath.Join(dir, "config.toml")
		configContent := `version = 2

[old-name]
project_id = "test_project"
service = "cloud"
`
		err := os.WriteFile(configPath, []byte(configContent), 0600)
		require.NoError(t, err)

		profile, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		err = config.InitProfile("old-name")
		require.NoError(t, err)
		assert.Equal(t, "old-name", profile.Name())

		// Rename profile
		err = profile.Rename("new-name")
		require.NoError(t, err)

		// Load config again
		profile, err = config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		profiles := config.List()
		assert.Contains(t, profiles, "new-name")
		assert.NotContains(t, profiles, "old-name")
	})
}
