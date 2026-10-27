// Copyright 2026 MongoDB Inc
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
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func tempConfigHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	dir, err := os.UserConfigDir()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, AtlasCLI), 0o700))
}

// TestFreshRefreshToken verifies the stored refresh token is read from a new store,
// not from the caller's in-memory snapshot, so a rotation by another process is seen.
func TestFreshRefreshToken(t *testing.T) {
	tempConfigHome(t)
	keyring.MockInit()

	snapshotStore, err := NewStoreWithEnvOption(false)
	require.NoError(t, err)
	require.True(t, snapshotStore.IsSecure())
	snapshot := NewProfile(DefaultProfile, snapshotStore)
	snapshot.SetAuthType(UserAccount)
	snapshot.SetRefreshToken("original-refresh-token")
	require.NoError(t, snapshot.Save())

	// Another process rotates the token and persists it.
	otherStore, err := NewStoreWithEnvOption(false)
	require.NoError(t, err)
	other := NewProfile(DefaultProfile, otherStore)
	other.SetRefreshToken("rotated-refresh-token")
	require.NoError(t, other.Save())

	assert.Equal(t, "original-refresh-token", snapshot.RefreshToken(), "the snapshot is not reloaded")
	fresh, err := snapshot.FreshRefreshToken()
	require.NoError(t, err)
	assert.Equal(t, "rotated-refresh-token", fresh)
}

// TestFreshRefreshTokenSecureStorageUnavailable verifies a fresh read that loses
// secure storage errors instead of reporting no stored token.
func TestFreshRefreshTokenSecureStorageUnavailable(t *testing.T) {
	tempConfigHome(t)
	keyring.MockInit()

	store, err := NewStoreWithEnvOption(false)
	require.NoError(t, err)
	require.True(t, store.IsSecure())
	profile := NewProfile(DefaultProfile, store)
	profile.SetRefreshToken("refresh-token")
	require.NoError(t, profile.Save())

	keyring.MockInitWithError(errors.New("keychain locked"))
	t.Cleanup(keyring.MockInit)

	fresh, err := profile.FreshRefreshToken()
	require.Error(t, err)
	assert.Empty(t, fresh)
}
