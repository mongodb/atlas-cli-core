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
