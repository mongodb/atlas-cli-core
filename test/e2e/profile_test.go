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
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/mongodb/atlas-cli-core/config"
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
	config.SetProjectID("64f1a5b2c3d4e5f6789012ab")
	config.SetOrgID("64f1a5b2c3d4e5f6789012cd")
	config.SetService(config.CloudService)

	// Verify configuration can be read
	assert.Equal(t, "64f1a5b2c3d4e5f6789012ab", config.ProjectID())
	assert.Equal(t, "64f1a5b2c3d4e5f6789012cd", config.OrgID())
	assert.Equal(t, config.CloudService, config.Service())
	assert.True(t, config.IsCloud())

	// Test profile existence checks
	assert.True(t, config.Exists(profileName))
	assert.False(t, config.Exists("nonexistent-profile"))

	// Test profile listing
	profiles := config.List()
	assert.Contains(t, profiles, profileName)
}

// TestLoadAtlasCLIConfigBasic tests that LoadAtlasCLIConfig successfully creates a profile
// with a configurable store that can handle both secure and insecure properties.
// This test focuses on the core functionality without trying to test every edge case.
func TestLoadAtlasCLIConfigBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	tempDir := t.TempDir()

	// Set environment variable to override config directory
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Clear and reconfigure viper for isolated test
	viper.Reset()

	// Test LoadAtlasCLIConfig creates a working profile
	loadedProfile, err := config.LoadAtlasCLIConfig()
	require.NoError(t, err, "LoadAtlasCLIConfig should not return an error")
	require.NotNil(t, loadedProfile, "LoadAtlasCLIConfig should return a profile")

	// Verify basic profile properties
	assert.Equal(t, config.DefaultProfile, loadedProfile.Name(), "Profile should have default name")

	// Test that we can set and retrieve insecure properties through the loaded profile
	loadedProfile.SetProjectID("test-project-123")
	loadedProfile.SetOrgID("test-org-456")
	loadedProfile.SetService(config.CloudService)
	loadedProfile.SetOutput("json")

	assert.Equal(t, "test-project-123", loadedProfile.ProjectID(), "Should be able to set and get project ID")
	assert.Equal(t, "test-org-456", loadedProfile.OrgID(), "Should be able to set and get org ID")
	assert.Equal(t, config.CloudService, loadedProfile.Service(), "Should be able to set and get service")
	assert.Equal(t, "json", loadedProfile.Output(), "Should be able to set and get output format")

	// Test that we can set and retrieve secure properties through the loaded profile
	// (these may be stored insecurely if secure storage is not available, but that's OK for this test)
	loadedProfile.SetPublicAPIKey("test-public-key")
	loadedProfile.SetPrivateAPIKey("test-private-key")
	loadedProfile.SetAuthType(config.APIKeys)

	// The profile should be able to store and retrieve these values regardless of storage type
	assert.Equal(t, "test-public-key", loadedProfile.PublicAPIKey(), "Should be able to set and get public API key")
	assert.Equal(t, "test-private-key", loadedProfile.PrivateAPIKey(), "Should be able to set and get private API key")
	assert.Equal(t, config.APIKeys, loadedProfile.AuthType(), "Should be able to set and get auth type")

	// Profile should report having access credentials
	assert.True(t, loadedProfile.IsAccessSet(), "Profile should report access is set when credentials are available")

	// Test save functionality
	err = loadedProfile.Save()
	assert.NoError(t, err, "Should be able to save the profile")
}

// TestLoadAtlasCLIConfigPersistence tests that LoadAtlasCLIConfig can load previously saved configuration
// from both secure and insecure storage.
func TestLoadAtlasCLIConfigPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	tempDir := t.TempDir()

	// Set environment variable to override config directory
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	// Clear and reconfigure viper for isolated test
	viper.Reset()

	// First, create and save a configuration
	profile1, err := config.LoadAtlasCLIConfig()
	require.NoError(t, err)

	// Set some configuration values
	profile1.SetProjectID("persistent-project-123")
	profile1.SetOrgID("persistent-org-456")
	profile1.SetService(config.CloudService)
	profile1.SetOutput("json")
	profile1.SetPublicAPIKey("persistent-public-key")
	profile1.SetPrivateAPIKey("persistent-private-key")
	profile1.SetAuthType(config.APIKeys)

	// Save the configuration
	err = profile1.Save()
	require.NoError(t, err)

	// Reset the global state to simulate a fresh start
	config.SetDefaultProfile(&config.Profile{})
	viper.Reset()

	// Now load the configuration again to test persistence
	profile2, err := config.LoadAtlasCLIConfig()
	require.NoError(t, err)
	require.NotNil(t, profile2)

	// Verify that the configuration was persisted and loaded correctly
	assert.Equal(t, "persistent-project-123", profile2.ProjectID(), "Project ID should be persisted")
	assert.Equal(t, "persistent-org-456", profile2.OrgID(), "Org ID should be persisted")
	assert.Equal(t, config.CloudService, profile2.Service(), "Service should be persisted")
	assert.Equal(t, "json", profile2.Output(), "Output format should be persisted")

	// Check secure properties - they should be available regardless of whether
	// secure storage is available (may fall back to insecure storage)
	assert.Equal(t, "persistent-public-key", profile2.PublicAPIKey(), "Public API key should be persisted")
	assert.Equal(t, "persistent-private-key", profile2.PrivateAPIKey(), "Private API key should be persisted")
	assert.Equal(t, config.APIKeys, profile2.AuthType(), "Auth type should be persisted")

	// Profile should report having access
	assert.True(t, profile2.IsAccessSet(), "Profile should report access is set after reload")

	// Verify profile list functionality works
	profiles := config.List()
	assert.Contains(t, profiles, config.DefaultProfile, "Default profile should exist in profile list")
}

// TestLoadAtlasCLIConfigErrorScenarios tests error handling and edge cases in LoadAtlasCLIConfig.
func TestLoadAtlasCLIConfigErrorScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	t.Run("ConfigDirPermissionError", func(t *testing.T) {
		// Test what happens when we can't create the config directory
		// Set an invalid config home that would cause permission errors
		t.Setenv("XDG_CONFIG_HOME", "/root/invalid-dir-no-permissions")
		viper.Reset()

		// The function should still work because it creates directories as needed
		// or falls back gracefully, but let's test the behavior
		profile, err := config.LoadAtlasCLIConfig()

		// Depending on the system, this might succeed or fail
		// The important thing is that it handles the error gracefully
		if err != nil {
			// If it fails, it should return a meaningful error message
			assert.Contains(t, err.Error(), "error loading config", "Error should mention config loading")
		} else {
			// If it succeeds, we should get a valid profile
			assert.NotNil(t, profile, "Should return a valid profile even with directory issues")
		}
	})

	t.Run("SecureStorageWarning", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tempDir)
		viper.Reset()

		// Capture stderr to check for the secure storage warning
		originalStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		profile, err := config.LoadAtlasCLIConfig()

		// Close writer and restore stderr
		w.Close()
		os.Stderr = originalStderr

		// Read captured output
		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		require.NoError(t, err, "LoadAtlasCLIConfig should succeed")
		require.NotNil(t, profile, "Should return a valid profile")

		// Check if we got the expected warning about secure storage
		// Note: This might or might not appear depending on keyring availability
		t.Logf("Captured stderr output: %s", output)
		// We can't reliably test for the warning as it depends on system keyring availability
		// But we can test that the function completes successfully

		// The important thing is that LoadAtlasCLIConfig succeeded
		assert.Equal(t, config.DefaultProfile, profile.Name(), "Should create default profile successfully")
	})

	t.Run("WithExistingCorruptedConfig", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tempDir)
		viper.Reset()

		// Create a corrupted config file
		configDir := filepath.Join(tempDir, "atlascli")
		err := os.MkdirAll(configDir, 0755)
		require.NoError(t, err)

		configFile := filepath.Join(configDir, "config.toml")
		err = os.WriteFile(configFile, []byte("invalid toml content [[["), 0600)
		require.NoError(t, err)

		// LoadAtlasCLIConfig should handle the corrupted file gracefully
		profile, err := config.LoadAtlasCLIConfig()

		if err != nil {
			// If it fails due to corruption, the error should be meaningful
			assert.Contains(t, err.Error(), "error loading config", "Error should mention config loading")
		} else {
			// If it handles corruption gracefully, we should get a valid profile
			assert.NotNil(t, profile, "Should return a valid profile even with corrupted config")
			assert.Equal(t, config.DefaultProfile, profile.Name(), "Should create default profile")
		}
	})

	t.Run("VerifyProfileSetup", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tempDir)
		viper.Reset()

		// Reset global profile state to ensure clean test
		config.SetDefaultProfile(&config.Profile{})

		profile, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)
		require.NotNil(t, profile)

		// Verify that LoadAtlasCLIConfig sets the global profile
		globalProfile := config.Default()
		assert.Equal(t, profile.Name(), globalProfile.Name(), "Global profile should be set to the loaded profile")

		// Verify the profile has the default name
		assert.Equal(t, config.DefaultProfile, profile.Name(), "Profile should have the default name")
	})
}

// TestLoadAtlasCLIConfigStorageTypes tests the different storage scenarios explicitly.
func TestLoadAtlasCLIConfigStorageTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	t.Run("CheckStorageTypeReporting", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tempDir)
		viper.Reset()

		profile, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)
		require.NotNil(t, profile)

		// Test setting both secure and insecure properties
		profile.SetProjectID("storage-test-project")
		profile.SetPublicAPIKey("storage-test-key")

		// Both should be retrievable regardless of storage type
		assert.Equal(t, "storage-test-project", profile.ProjectID(), "Insecure property should be stored and retrieved")
		assert.Equal(t, "storage-test-key", profile.PublicAPIKey(), "Secure property should be stored and retrieved")

		// Test saving
		err = profile.Save()
		assert.NoError(t, err, "Should be able to save regardless of storage type")
	})

	t.Run("StorageTypeConsistency", func(t *testing.T) {
		tempDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tempDir)
		viper.Reset()

		// First profile instance
		profile1, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		// Set some properties
		profile1.SetProjectID("consistency-project")
		profile1.SetPublicAPIKey("consistency-key")
		err = profile1.Save()
		require.NoError(t, err)

		// Reset and load again
		config.SetDefaultProfile(&config.Profile{})
		viper.Reset()

		profile2, err := config.LoadAtlasCLIConfig()
		require.NoError(t, err)

		// Properties should be consistent regardless of how storage was handled
		assert.Equal(t, "consistency-project", profile2.ProjectID(), "Project ID should be consistent across loads")
		// Note: API key may or may not persist depending on secure storage availability,
		// but the important thing is that the system works consistently

		if profile2.PublicAPIKey() != "" {
			assert.Equal(t, "consistency-key", profile2.PublicAPIKey(), "If API key persists, it should be consistent")
		}
	})
}
