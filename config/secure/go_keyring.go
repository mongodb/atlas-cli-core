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

// This file implements KeyringStore for secure credential storage using the system
// keyring, with in-memory caching and deferred persistence through pending operations.
// Each credential saved on the keyring is connected to a service. Each service
// represents a config profile.
//
// Example:
// config.toml profile:
// -> profile: "default", publicAPIKey: "pubKey", privateAPIKey: "privKey"
// Becomes keyring:
// -> service: atlascli_default, username: "pubKey", password: "privKey"

package secure

import (
	"errors"
	"slices"

	"github.com/zalando/go-keyring"
)

//go:generate go tool go.uber.org/mock/mockgen -destination=../../mocks/mock_go_keyring.go -package=mocks github.com/mongodb/atlas-cli-core/config/secure KeyringClient

const servicePrefix = "atlascli_"

// createServiceName generates a keyring service name with profile-specific prefix.
func createServiceName(profileName string) string {
	return servicePrefix + profileName
}

// KeyringClient abstracts keyring operations for easier testing.
type KeyringClient interface {
	Set(service, user, password string) error
	Get(service, user string) (string, error)
	Delete(service, user string) error
	DeleteAll(service string) error
}

// DefaultKeyringClient implements KeyringClient using the zalando/go-keyring library.
type DefaultKeyringClient struct{}

// NewDefaultKeyringClient creates a new DefaultKeyringClient instance.
func NewDefaultKeyringClient() *DefaultKeyringClient {
	return &DefaultKeyringClient{}
}

// Set stores a credential in the keyring.
func (*DefaultKeyringClient) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

// Get retrieves a credential from the keyring, returning empty string if not found.
func (*DefaultKeyringClient) Get(service, user string) (string, error) {
	value, err := keyring.Get(service, user)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return "", err
	}

	return value, nil
}

// Delete removes a credential from the keyring.
func (*DefaultKeyringClient) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

// DeleteAll removes all credentials for a service from the keyring.
func (*DefaultKeyringClient) DeleteAll(service string) error {
	return keyring.DeleteAll(service)
}

// Operation types for tracking changes.
type operationType int

const (
	opSet operationType = iota
	opDelete
	opDeleteProfile
)

// pendingOperation represents a change that needs to be persisted.
type pendingOperation struct {
	opType       operationType
	profileName  string
	propertyName string
	value        string
}

// KeyringStore provides secure storage using the system keyring with in-memory
// caching. Set and delete operations are cached for a later time to keep in
// alignment with viper behavior which is to store all properties in memory
// until they are explicitly saved.
type KeyringStore struct {
	// Available indicates if the keyring is available.
	available bool
	// In-memory cache: map[profileName]map[propertyName]value
	cache map[string]map[string]string
	// List of operations to perform when Save() is called
	pendingOps []pendingOperation
	// Properties that are considered secure
	secureProperties []string
	// KeyringClient for keyring operations
	keyringClient KeyringClient
}

// NewSecureStore creates a KeyringStore with default keyring client.
func NewSecureStore(profileNames []string, secureProperties []string) *KeyringStore {
	return NewSecureStoreWithClient(profileNames, secureProperties, NewDefaultKeyringClient())
}

// NewSecureStoreWithClient creates a KeyringStore with custom keyring client.
func NewSecureStoreWithClient(profileNames []string, secureProperties []string, keyringClient KeyringClient) *KeyringStore {
	store := &KeyringStore{
		cache:            make(map[string]map[string]string),
		pendingOps:       make([]pendingOperation, 0),
		secureProperties: secureProperties,
		keyringClient:    keyringClient,
	}

	// Check if the keyring is available.
	// We do this by marking the store as available if we can get a value from the keyring.
	available := false
	attemptedToRead := false

	// Load all existing secure properties for all profiles into memory
outer:
	for _, profileName := range profileNames {
		store.cache[profileName] = make(map[string]string)
		for _, propertyName := range secureProperties {
			attemptedToRead = true

			// Attempt to read the value from the keyring.
			value, err := keyringClient.Get(createServiceName(profileName), propertyName)

			// If the store returns an error, break the loop.
			if err != nil {
				break outer
			}

			store.cache[profileName][propertyName] = value
			available = true
		}
	}

	// If we didn't attempt to read, try to read a value from the default service.
	if !attemptedToRead {
		_, err := keyringClient.Get(createServiceName("default"), "test")
		available = err == nil
	}

	// Set the available flag.
	store.available = available

	return store
}

// Available returns whether secure storage is available.
func (k *KeyringStore) Available() bool {
	return k.available
}

// Save executes all pending operations to the keyring.
func (k *KeyringStore) Save() error {
	// Process all pending operations
	for _, op := range k.pendingOps {
		switch op.opType {
		case opSet:
			if err := k.keyringClient.Set(createServiceName(op.profileName), op.propertyName, op.value); err != nil {
				return err
			}
		case opDelete:
			if err := k.keyringClient.Delete(createServiceName(op.profileName), op.propertyName); err != nil {
				return err
			}
		case opDeleteProfile:
			if err := k.keyringClient.DeleteAll(createServiceName(op.profileName)); err != nil {
				return err
			}
		}
	}

	// Clear pending operations after successful save
	k.pendingOps = make([]pendingOperation, 0)
	return nil
}

// Set stores a value in cache and queues it to later be saved.
func (k *KeyringStore) Set(profileName string, propertyName string, value string) {
	// Ignore properties that are not in SecureProperties
	if !slices.Contains(k.secureProperties, propertyName) {
		return
	}

	// Initialize profile map if it doesn't exist
	if k.cache[profileName] == nil {
		k.cache[profileName] = make(map[string]string)
	}

	// Update in-memory cache
	k.cache[profileName][propertyName] = value

	// Add to pending operations
	k.pendingOps = append(k.pendingOps, pendingOperation{
		opType:       opSet,
		profileName:  profileName,
		propertyName: propertyName,
		value:        value,
	})
}

// Get retrieves a value from the in-memory cache.
func (k *KeyringStore) Get(profileName string, propertyName string) string {
	// Check if profile exists in cache
	if profileCache, exists := k.cache[profileName]; exists {
		if value, exists := profileCache[propertyName]; exists {
			return value
		}
	}
	return ""
}

// DeleteKey removes a property from cache and queues keyring deletion.
func (k *KeyringStore) DeleteKey(profileName string, propertyName string) {
	// Remove from in-memory cache if it exists
	if profileCache, exists := k.cache[profileName]; exists {
		delete(profileCache, propertyName)
	}

	// Add to pending operations
	k.pendingOps = append(k.pendingOps, pendingOperation{
		opType:       opDelete,
		profileName:  profileName,
		propertyName: propertyName,
	})
}

// DeleteProfile removes a profile from cache and queues keyring deletion.
func (k *KeyringStore) DeleteProfile(profileName string) {
	// Remove from in-memory cache
	delete(k.cache, profileName)

	// Add to pending operations
	k.pendingOps = append(k.pendingOps, pendingOperation{
		opType:      opDeleteProfile,
		profileName: profileName,
	})
}
