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
type ProxyStore struct {
	insecure Store
	secure   SecureStore
}

// NewDefaultStore creates a store with default filesystem and secure storage if available.
func NewDefaultStore() (Store, error) {
	insecure, err := NewViperStore(afero.NewOsFs(), true)

	if err != nil {
		return nil, err
	}

	profileNames := insecure.GetProfileNames()
	secureStore := secure.NewSecureStore(profileNames, SecureProperties)

	return NewStore(insecure, secureStore), nil
}

// NewStore creates a ProxyStore if secure storage is available, otherwise returns insecure store.
func NewStore(insecureStore Store, secureStore SecureStore) Store {
	if !secureStore.Available() {
		return insecureStore
	}

	return &ProxyStore{
		insecure: insecureStore,
		secure:   secureStore,
	}
}

// isSecureProperty checks if a property requires secure storage.
func isSecureProperty(propertyName string) bool {
	return slices.Contains(SecureProperties, propertyName)
}

// Store interface implementation for ProxyStore

// IsSecure returns true as ProxyStore provides secure storage capabilities.
func (*ProxyStore) IsSecure() bool {
	return true
}

// Save persists both secure and insecure stores, collecting any errors.
func (p *ProxyStore) Save() error {
	errs := []error{}

	if err := p.insecure.Save(); err != nil {
		errs = append(errs, err)
	}

	if err := p.secure.Save(); err != nil {
		errs = append(errs, err)
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

// GetHierarchicalValue routes to secure or insecure store based on property type.
func (p *ProxyStore) GetHierarchicalValue(profileName string, propertyName string) any {
	if isSecureProperty(propertyName) {
		val := p.secure.Get(profileName, propertyName)
		if val != "" {
			return val
		}
	}
	return p.insecure.GetHierarchicalValue(profileName, propertyName)
}

// SetProfileValue routes to secure or insecure store based on property type.
func (p *ProxyStore) SetProfileValue(profileName string, propertyName string, value any) {
	if isSecureProperty(propertyName) {
		if v, ok := value.(string); ok {
			p.secure.Set(profileName, propertyName, v)
		}
		return
	}
	p.insecure.SetProfileValue(profileName, propertyName, value)
}

// GetProfileValue routes to secure or insecure store based on property type.
func (p *ProxyStore) GetProfileValue(profileName string, propertyName string) any {
	if isSecureProperty(propertyName) {
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
	if isSecureProperty(propertyName) {
		if v, ok := value.(string); ok {
			p.secure.Set(DefaultProfile, propertyName, v)
		}
		return
	}
	p.insecure.SetGlobalValue(propertyName, value)
}

// GetGlobalValue routes to secure or insecure store based on property type.
func (p *ProxyStore) GetGlobalValue(propertyName string) any {
	if isSecureProperty(propertyName) {
		return p.secure.Get(DefaultProfile, propertyName)
	}
	return p.insecure.GetGlobalValue(propertyName)
}

// IsSetGlobal checks only insecure store for global property existence as
// no secure properties are global
func (p *ProxyStore) IsSetGlobal(propertyName string) bool {
	return p.insecure.IsSetGlobal(propertyName)
}
