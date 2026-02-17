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

// This file implements ProxyStore which routes configuration properties between
// secure and insecure storage based on property type, providing unified access.
// If secure storage is not available, insecure storage is used directly without
// ProxyStore as a wrapper.

package config

import (
	"errors"
	"slices"

	"github.com/mongodb/atlas-cli-core/config/secure"
	"github.com/spf13/afero"
)

// SecureProperties defines which configuration properties require secure storage.
var SecureProperties = []string{
	publicAPIKey,
	privateAPIKey,
	AccessTokenField,
	RefreshTokenField,
	ClientIDField,
	ClientSecretField,
}

// ProxyStore routes properties between secure and insecure stores based on property type.
// It implements a three-layer priority system for secure properties:
// 1. Environment variables (highest priority)
// 2. Secure store (keyring)
// 3. Config file (lowest priority)
// Both environment and secure are nil when not available.
type ProxyStore struct {
	environment Store       // Viper with no filesystem, only environment variables; nil if not loaded
	insecure    Store       // Viper with filesystem, no environment variables
	secure      SecureStore // System keyring; nil if not available
}

// NewDefaultStore creates a store with default filesystem and secure storage if available.
func NewDefaultStore() (Store, error) {
	return NewStoreWithEnvOption(true)
}

// NewStoreWithEnvOption creates a store with default filesystem and secure storage
// if available. It will load environment variables according to the loadEnvVars input.
func NewStoreWithEnvOption(loadEnvVars bool) (Store, error) {
	// Create file-based store (no env vars)
	insecureStore, err := NewViperStore(afero.NewOsFs(), false)
	if err != nil {
		return nil, err
	}

	var environmentStore Store
	if loadEnvVars {
		// Create environment-only store (in-memory filesystem, no config file)
		environmentStore, err = NewViperStore(afero.NewMemMapFs(), true)
		if err != nil {
			return nil, err
		}
	}

	profileNames := insecureStore.GetProfileNames()
	secureStore := secure.NewSecureStore(profileNames, SecureProperties)

	return NewStore(environmentStore, insecureStore, secureStore), nil
}

// NewStore creates a ProxyStore if we have environment or secure storage,
// otherwise returns insecure store directly.
func NewStore(environment Store, insecure Store, secureStore SecureStore) Store {
	var secure SecureStore
	if secureStore != nil && secureStore.Available() {
		secure = secureStore
	}

	if environment == nil && secure == nil {
		return insecure
	}

	return &ProxyStore{
		environment: environment,
		insecure:    insecure,
		secure:      secure,
	}
}

// isSecureProperty checks if a property requires secure storage.
func isSecureProperty(propertyName string) bool {
	return slices.Contains(SecureProperties, propertyName)
}

// Store interface implementation for ProxyStore

// IsSecure returns true if the secure store (keyring) is available.
func (p *ProxyStore) IsSecure() bool {
	return p.secure != nil
}

// Save persists both secure and insecure stores, collecting any errors.
// Environment store is not persisted as it's read-only.
func (p *ProxyStore) Save() error {
	errs := []error{}

	if err := p.insecure.Save(); err != nil {
		errs = append(errs, err)
	}

	if p.secure != nil {
		if err := p.secure.Save(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// GetProfileNames returns profile names from the insecure store.
func (p *ProxyStore) GetProfileNames() []string {
	return p.insecure.GetProfileNames()
}

// RenameProfile delegates to the insecure store for profile management.
func (p *ProxyStore) RenameProfile(oldProfileName string, newProfileName string) error {
	return p.insecure.RenameProfile(oldProfileName, newProfileName)
}

// DeleteProfile delegates to the insecure store for profile management.
func (p *ProxyStore) DeleteProfile(profileName string) error {
	return p.insecure.DeleteProfile(profileName)
}

// GetHierarchicalValue implements three-layer priority for secure properties:
// 1. Environment variables (highest priority)
// 2. Secure store (keyring)
// 3. Config file (lowest priority)
// For non-secure properties, checks env vars first, then config file.
func (p *ProxyStore) GetHierarchicalValue(profileName string, propertyName string) any {
	// Layer 1: Check environment variables first (if environment store is available)
	if p.environment != nil {
		if envVal := p.environment.GetHierarchicalValue(profileName, propertyName); envVal != nil && envVal != "" {
			return envVal
		}
	}

	// For secure properties, check secure store before insecure store
	if isSecureProperty(propertyName) && p.secure != nil {
		// Layer 2: Check secure store (keyring)
		if secureVal := p.secure.Get(profileName, propertyName); secureVal != "" {
			return secureVal
		}
	}

	// Layer 3: Fall back to insecure config file
	return p.insecure.GetHierarchicalValue(profileName, propertyName)
}

// SetProfileValue routes to secure or insecure store based on property type.
func (p *ProxyStore) SetProfileValue(profileName string, propertyName string, value any) {
	if isSecureProperty(propertyName) && p.secure != nil {
		if v, ok := value.(string); ok {
			p.secure.Set(profileName, propertyName, v)
		}
		return
	}
	p.insecure.SetProfileValue(profileName, propertyName, value)
}

// GetProfileValue routes to secure or insecure store based on property type.
func (p *ProxyStore) GetProfileValue(profileName string, propertyName string) any {
	if isSecureProperty(propertyName) && p.secure != nil {
		return p.secure.Get(profileName, propertyName)
	}
	return p.insecure.GetProfileValue(profileName, propertyName)
}

// GetProfileStringMap returns insecure properties only, excluding secure values.
func (p *ProxyStore) GetProfileStringMap(profileName string) map[string]string {
	return p.insecure.GetProfileStringMap(profileName)
}

// SetGlobalValue routes to secure or insecure store based on property type.
func (p *ProxyStore) SetGlobalValue(propertyName string, value any) {
	if isSecureProperty(propertyName) && p.secure != nil {
		if v, ok := value.(string); ok {
			p.secure.Set(DefaultProfile, propertyName, v)
		}
		return
	}
	p.insecure.SetGlobalValue(propertyName, value)
}

// GetGlobalValue implements three-layer priority for secure properties:
// 1. Environment variables (highest priority)
// 2. Secure store (keyring)
// 3. Config file (lowest priority)
func (p *ProxyStore) GetGlobalValue(propertyName string) any {
	// Layer 1: Check environment variables first (if environment store is available)
	if p.environment != nil {
		if envVal := p.environment.GetGlobalValue(propertyName); envVal != nil && envVal != "" {
			return envVal
		}
	}

	// For secure properties, check secure store before insecure store
	if isSecureProperty(propertyName) && p.secure != nil {
		// Layer 2: Check secure store (keyring)
		if secureVal := p.secure.Get(DefaultProfile, propertyName); secureVal != "" {
			return secureVal
		}
	}

	// Layer 3: Fall back to insecure config file
	return p.insecure.GetGlobalValue(propertyName)
}

// IsSetGlobal checks if a global property is set in any store.
func (p *ProxyStore) IsSetGlobal(propertyName string) bool {
	if p.environment != nil && p.environment.IsSetGlobal(propertyName) {
		return true
	}
	return p.insecure.IsSetGlobal(propertyName)
}
