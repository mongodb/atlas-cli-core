// Copyright 2025 MongoDB Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file defines the core Store and SecureStore interfaces, along with an
// InMemoryStore implementation that serves as a temporary workaround to maintain
// compatibility with legacy unit tests that expect profiles to be available immediately.

package config

import (
	"slices"
	"sort"

	"github.com/spf13/viper"
)

//go:generate go tool go.uber.org/mock/mockgen -destination=../mocks/mock_store.go -package=mocks github.com/mongodb/atlas-cli-core/config Store,SecureStore

type Store interface {
	IsSecure() bool
	Save() error

	GetProfileNames() []string
	RenameProfile(oldProfileName string, newProfileName string) error
	DeleteProfile(profileName string) error

	GetHierarchicalValue(profileName string, propertyName string) any

	SetProfileValue(profileName string, propertyName string, value any)
	GetProfileValue(profileName string, propertyName string) any
	GetProfileStringMap(profileName string) map[string]string

	SetGlobalValue(propertyName string, value any)
	GetGlobalValue(propertyName string) any
	IsSetGlobal(propertyName string) bool
}

type SecureStore interface {
	Available() bool
	Save() error

	Set(profileName string, propertyName string, value string)
	Get(profileName string, propertyName string) string
	DeleteKey(profileName string, propertyName string)
	DeleteProfile(profileName string)
}

// Temporary InMemoryStore to mimick legacy behavior
// Will be removed by CLOUDP-339855 when we get rid of static references in the profile
type InMemoryStore struct {
	v *viper.Viper
}

// NewInMemoryStore creates a new InMemoryStore instance with an initialized Viper configuration.
// This store is used as a temporary workaround for unit tests that expect profiles to be
// available before they are actually configured, preventing nil pointer references.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		v: viper.New(),
	}
}

// IsSecure returns true to indicate this store treats data as secure, even though
// it's stored in memory. This maintains compatibility with the Store interface.
func (*InMemoryStore) IsSecure() bool {
	return true
}

// Save is a no-op for InMemoryStore since data is only stored in memory and
// doesn't need to be persisted to disk.
func (*InMemoryStore) Save() error {
	return nil
}

// GetProfileNames returns a sorted list of all profile names stored in the configuration.
func (s *InMemoryStore) GetProfileNames() []string {
	allKeys := s.v.AllSettings()

	profileNames := make([]string, 0, len(allKeys))
	for key := range allKeys {
		if !slices.Contains(AllProperties(), key) {
			profileNames = append(profileNames, key)
		}
	}
	// keys in maps are non-deterministic, trying to give users a consistent output
	sort.Strings(profileNames)
	return profileNames
}

// RenameProfile is not implemented for InMemoryStore and will panic if called.
// This functionality is intended for persistent existing tests.
func (*InMemoryStore) RenameProfile(_, _ string) error {
	panic("not implemented")
}

// DeleteProfile is not implemented for InMemoryStore and will panic if called.
// This functionality is intended for persistent existing tests.
func (*InMemoryStore) DeleteProfile(_ string) error {
	panic("not implemented")
}

// GetHierarchicalValue retrieves a property value with hierarchical precedence.
// It first checks for global properties, then falls back to profile-specific settings.
func (s *InMemoryStore) GetHierarchicalValue(profileName string, propertyName string) any {
	if s.v.IsSet(propertyName) && s.v.Get(propertyName) != "" {
		return s.v.Get(propertyName)
	}
	settings := s.v.GetStringMap(profileName)
	return settings[propertyName]
}

// SetProfileValue sets a property value for a specific profile, creating or updating
// the profile's configuration map as needed.
func (s *InMemoryStore) SetProfileValue(profileName string, propertyName string, value any) {
	settings := s.v.GetStringMap(profileName)
	settings[propertyName] = value
	s.v.Set(profileName, settings)
}

// GetProfileValue retrieves a property value from a specific profile's configuration.
func (s *InMemoryStore) GetProfileValue(profileName string, propertyName string) any {
	settings := s.v.GetStringMap(profileName)
	return settings[propertyName]
}

// GetProfileStringMap returns all configuration properties for a profile as a string map.
func (s *InMemoryStore) GetProfileStringMap(profileName string) map[string]string {
	return s.v.GetStringMapString(profileName)
}

// SetGlobalValue sets a global configuration property that applies across all profiles.
func (s *InMemoryStore) SetGlobalValue(propertyName string, value any) {
	s.v.Set(propertyName, value)
}

// GetGlobalValue retrieves a global configuration property value.
func (s *InMemoryStore) GetGlobalValue(propertyName string) any {
	return s.v.Get(propertyName)
}

// IsSetGlobal checks whether a global configuration property has been set.
func (s *InMemoryStore) IsSetGlobal(propertyName string) bool {
	return s.v.IsSet(propertyName)
}
