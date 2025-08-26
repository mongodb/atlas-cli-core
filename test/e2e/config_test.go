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
	"testing"

	"github.com/mongodb/atlas-cli-core/config"
	"github.com/mongodb/atlas-cli-core/test/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigLoadingE2E tests the complete config loading flow in a real environment
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
		assert.Empty(t, profile.OrgID())
		assert.Empty(t, profile.PublicAPIKey())
		assert.Empty(t, profile.Service())
	})

	t.Run("LoadAtlasCLIConfigWithVersion(2) succeeds", func(t *testing.T) {
		withNewConfig(t)
		profile, err := config.LoadAtlasCLIConfigWithVersion(2)
		require.NoError(t, err)
		require.NotNil(t, profile)
		assert.Equal(t, "default", profile.Name())
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
  version = 2
  [config_test]
  org_id = "new_config_org_id"
  public_api_key = "new_config_pub"
  service = "cloud"
`), 0600)
	require.NoError(t, err)
}
