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
	"bytes"
	"fmt"
	"os"
	"path"
)

// MaxSupportedVersion is the maximum supported config version available in AtlasCLI.
const MaxSupportedVersion = 2

// CLIConfigHome retrieves configHome path.
func CLIConfigHome() (string, error) {
	home, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return path.Join(home, "atlascli"), nil
}

func Path(f string) (string, error) {
	var p bytes.Buffer

	h, err := CLIConfigHome()
	if err != nil {
		return "", err
	}

	p.WriteString(h)
	p.WriteString(f)
	return p.String(), nil
}

// LoadAtlasCLIConfig loads configuration using the maximum supported version,
// ensuring plugins always use the latest AtlasCLI configuration schema.
func LoadAtlasCLIConfig() (*Profile, error) {
	return LoadAtlasCLIConfigWithVersion(MaxSupportedVersion)
}

// LoadAtlasCLIConfigWithVersion loads configuration with validation for specified
// version. The expectedVersion parameter enforces compatibility by rejecting
// configs that are newer or older than expected.
func LoadAtlasCLIConfigWithVersion(expectedVersion int64) (*Profile, error) {
	configStore, initErr := NewDefaultStore()
	if initErr != nil {
		return nil, fmt.Errorf("error loading config: %w. Please run `atlas auth login` to reconfigure your profile", initErr)
	}

	if err := verifyConfigVersion(expectedVersion, configStore); err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}

	if !configStore.IsSecure() && !SilenceStorageWarning() {
		fmt.Fprintf(os.Stderr, `Warning: Secure storage is not available, falling back to insecure storage
		
		To disable this alert, run "atlas config set silence_storage_warning true"`)
	}

	profile := NewProfile(DefaultProfile, configStore)
	SetProfile(profile)

	return profile, nil
}

// verifyConfigVersion checks if the config version is as expected. This ensures
// an incompatible version of the config is not loaded and provides useful error
// messaging to users.
func verifyConfigVersion(expectedVersion int64, s Store) error {
	// check if there are any profiles set, if no profile is set, there is no configuration to verify
	profiles := s.GetProfileNames()
	if len(profiles) == 0 {
		return nil
	}

	if !s.IsSetGlobal("version") {
		// Scenario 1: User upgrades plugin but not AtlasCLI.
		return fmt.Errorf("config version is missing, expected version %d. Please upgrade to a newer version of AtlasCLI", expectedVersion)
	}

	rawVersion := s.GetGlobalValue("version")
	version, ok := rawVersion.(int64)
	if !ok {
		return fmt.Errorf("invalid config version type: %T", rawVersion)
	}
	// If version is greater than expectedVersion, the plugin is outdated.
	if version > expectedVersion {
		// Scenario 2: Plugin expects outdated configuration version and should be updated.
		return fmt.Errorf("config version %d is newer than expected version %d. Please upgrade to a newer version of this plugin", version, expectedVersion)
	}
	// If version is less than than expectedVersion, the AtlasCLI is outdated.
	if version < expectedVersion {
		// Scenario 3: User upgrades plugin but not AtlasCLI, which does not support expected configuration version. AtlasCLI should be updated.
		return fmt.Errorf("config version %d is older than expected version %d. Please upgrade to a newer version of AtlasCLI", version, expectedVersion)
	}

	return nil
}
