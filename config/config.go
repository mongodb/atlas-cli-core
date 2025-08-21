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

func LoadAtlasCLIConfig() (*Profile, error) {
	configStore, initErr := NewDefaultStore()

	if initErr != nil {
		return nil, fmt.Errorf("error loading config: %w. Please run `atlas auth login` to reconfigure your profile", initErr)
	}

	if !configStore.IsSecure() {
		fmt.Fprintf(os.Stderr, "Warning: Secure storage is not available, falling back to insecure storage\n")
	}

	profile := NewProfile(DefaultProfile, configStore)
	SetProfile(profile)

	return profile, nil
}
