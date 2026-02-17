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

package config

import (
	"testing"

	"github.com/mongodb/atlas-cli-core/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testProfileName = "test-profile"
	testValue       = "test-value"
	envVarTestValue = "env-var-value"
)

func TestNewStore(t *testing.T) {
	tests := []struct {
		name             string
		secureAvailable  bool
		hasEnvironment   bool
		expectProxyStore bool
	}{
		{
			name:             "secure store available with env - returns ProxyStore",
			secureAvailable:  true,
			hasEnvironment:   true,
			expectProxyStore: true,
		},
		{
			name:             "secure store available without env - returns ProxyStore",
			secureAvailable:  true,
			hasEnvironment:   false,
			expectProxyStore: true,
		},
		{
			name:             "secure store unavailable with env - returns ProxyStore",
			secureAvailable:  false,
			hasEnvironment:   true,
			expectProxyStore: true,
		},
		{
			name:             "secure store unavailable without env - returns insecure store",
			secureAvailable:  false,
			hasEnvironment:   false,
			expectProxyStore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			var environment Store
			if tt.hasEnvironment {
				environment = mocks.NewMockStore(ctrl)
			}
			insecure := mocks.NewMockStore(ctrl)
			mockSecure := mocks.NewMockSecureStore(ctrl)

			mockSecure.EXPECT().Available().Return(tt.secureAvailable).AnyTimes()

			store := NewStore(environment, insecure, mockSecure)

			if tt.expectProxyStore {
				proxyStore, ok := store.(*ProxyStore)
				require.True(t, ok, "Expected ProxyStore")
				assert.Equal(t, environment, proxyStore.environment)
				assert.Equal(t, insecure, proxyStore.insecure)
				if tt.secureAvailable {
					assert.Equal(t, mockSecure, proxyStore.secure)
				} else {
					assert.Nil(t, proxyStore.secure)
				}
			} else {
				assert.Equal(t, insecure, store)
			}
		})
	}
}

func TestProxyStore_IsSecure(t *testing.T) {
	ctrl := gomock.NewController(t)

	environment := mocks.NewMockStore(ctrl)
	insecure := mocks.NewMockStore(ctrl)
	mockSecure := mocks.NewMockSecureStore(ctrl)
	store := &ProxyStore{
		environment: environment,
		insecure:    insecure,
		secure:      mockSecure,
	}

	assert.True(t, store.IsSecure())
}

func TestProxyStore_PropertyRouting(t *testing.T) {
	testCases := []struct {
		propertyName string
		isSecure     bool
	}{
		{publicAPIKey, true},
		{privateAPIKey, true},
		{AccessTokenField, true},
		{RefreshTokenField, true},
		{"base_url", false},
		{"project_id", false},
		{"org_id", false},
		{"output", false},
		{"service", false},
	}

	methods := []struct {
		name     string
		testFunc func(t *testing.T, store *ProxyStore, propertyName string, isSecure bool)
	}{
		{
			name:     "GetHierarchicalValue",
			testFunc: testGetHierarchicalValue,
		},
		{
			name:     "SetProfileValue",
			testFunc: testSetProfileValue,
		},
		{
			name:     "GetProfileValue",
			testFunc: testGetProfileValue,
		},
		{
			name:     "SetGlobalValue",
			testFunc: testSetGlobalValue,
		},
		{
			name:     "GetGlobalValue",
			testFunc: testGetGlobalValue,
		},
	}

	for _, method := range methods {
		for _, tc := range testCases {
			t.Run(method.name+"_"+tc.propertyName, func(t *testing.T) {
				ctrl := gomock.NewController(t)

				environment := mocks.NewMockStore(ctrl)
				insecure := mocks.NewMockStore(ctrl)
				mockSecure := mocks.NewMockSecureStore(ctrl)

				store := &ProxyStore{
					environment: environment,
					insecure:    insecure,
					secure:      mockSecure,
				}

				method.testFunc(t, store, tc.propertyName, tc.isSecure)
			})
		}
	}
}

func testGetHierarchicalValue(t *testing.T, store *ProxyStore, propertyName string, isSecure bool) {
	t.Helper()
	profileName := testProfileName
	expectedValue := testValue

	if isSecure {
		// Three-layer priority for secure properties:
		// 1. Check environment variables (returns nil)
		store.environment.(*mocks.MockStore).EXPECT().
			GetHierarchicalValue(profileName, propertyName).
			Return(nil)
		// 2. Check secure store (returns value)
		store.secure.(*mocks.MockSecureStore).EXPECT().
			Get(profileName, propertyName).
			Return(expectedValue)
	} else {
		// For non-secure: check env (returns nil), then insecure store
		store.environment.(*mocks.MockStore).EXPECT().
			GetHierarchicalValue(profileName, propertyName).
			Return(nil)
		store.insecure.(*mocks.MockStore).EXPECT().
			GetHierarchicalValue(profileName, propertyName).
			Return(expectedValue)
	}

	result := store.GetHierarchicalValue(profileName, propertyName)
	assert.Equal(t, expectedValue, result)
}

func testSetProfileValue(t *testing.T, store *ProxyStore, propertyName string, isSecure bool) {
	t.Helper()
	profileName := testProfileName
	value := testValue

	if isSecure {
		store.secure.(*mocks.MockSecureStore).EXPECT().
			Set(profileName, propertyName, value)
	} else {
		store.insecure.(*mocks.MockStore).EXPECT().
			SetProfileValue(profileName, propertyName, value)
	}

	store.SetProfileValue(profileName, propertyName, value)
}

func testGetProfileValue(t *testing.T, store *ProxyStore, propertyName string, isSecure bool) {
	t.Helper()
	profileName := testProfileName
	expectedValue := testValue

	if isSecure {
		store.secure.(*mocks.MockSecureStore).EXPECT().
			Get(profileName, propertyName).
			Return(expectedValue)
	} else {
		store.insecure.(*mocks.MockStore).EXPECT().
			GetProfileValue(profileName, propertyName).
			Return(expectedValue)
	}

	result := store.GetProfileValue(profileName, propertyName)
	assert.Equal(t, expectedValue, result)
}

func testSetGlobalValue(t *testing.T, store *ProxyStore, propertyName string, isSecure bool) {
	t.Helper()
	value := testValue

	if isSecure {
		store.secure.(*mocks.MockSecureStore).EXPECT().
			Set(DefaultProfile, propertyName, value)
	} else {
		store.insecure.(*mocks.MockStore).EXPECT().
			SetGlobalValue(propertyName, value)
	}

	store.SetGlobalValue(propertyName, value)
}

func testGetGlobalValue(t *testing.T, store *ProxyStore, propertyName string, isSecure bool) {
	t.Helper()
	expectedValue := testValue

	if isSecure {
		// Three-layer priority for secure properties:
		// 1. Check environment variables (returns nil)
		store.environment.(*mocks.MockStore).EXPECT().
			GetGlobalValue(propertyName).
			Return(nil)
		// 2. Check secure store (returns value)
		store.secure.(*mocks.MockSecureStore).EXPECT().
			Get(DefaultProfile, propertyName).
			Return(expectedValue)
	} else {
		// For non-secure: check env (returns nil), then insecure store
		store.environment.(*mocks.MockStore).EXPECT().
			GetGlobalValue(propertyName).
			Return(nil)
		store.insecure.(*mocks.MockStore).EXPECT().
			GetGlobalValue(propertyName).
			Return(expectedValue)
	}

	result := store.GetGlobalValue(propertyName)
	assert.Equal(t, expectedValue, result)
}

func TestIsSecureProperty(t *testing.T) {
	tests := []struct {
		propertyName string
		expected     bool
	}{
		{publicAPIKey, true},
		{privateAPIKey, true},
		{AccessTokenField, true},
		{RefreshTokenField, true},
		{"base_url", false},
		{"project_id", false},
		{"org_id", false},
		{"output", false},
		{"", false},
		{"random_property", false},
	}

	for _, tt := range tests {
		t.Run(tt.propertyName, func(t *testing.T) {
			result := isSecureProperty(tt.propertyName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetHierarchicalValue_EnvVarPriority tests that environment variables
// take precedence over both secure store and insecure config for secure properties.
func TestGetHierarchicalValue_EnvVarPriority(t *testing.T) {
	ctrl := gomock.NewController(t)

	environment := mocks.NewMockStore(ctrl)
	insecure := mocks.NewMockStore(ctrl)
	mockSecure := mocks.NewMockSecureStore(ctrl)

	store := &ProxyStore{
		environment: environment,
		insecure:    insecure,
		secure:      mockSecure,
	}

	profileName := "test"

	// Test that env var value is returned (no secure or insecure store calls)
	environment.EXPECT().
		GetHierarchicalValue(profileName, ClientIDField).
		Return(envVarTestValue)

	result := store.GetHierarchicalValue(profileName, ClientIDField)
	assert.Equal(t, envVarTestValue, result)

	// Test fallback: env returns nil -> check secure store
	keyringValue := "keyring-value"
	environment.EXPECT().
		GetHierarchicalValue(profileName, ClientSecretField).
		Return(nil)
	mockSecure.EXPECT().
		Get(profileName, ClientSecretField).
		Return(keyringValue)

	result = store.GetHierarchicalValue(profileName, ClientSecretField)
	assert.Equal(t, keyringValue, result)

	// Test fallback: env returns nil, secure returns empty -> check insecure store
	insecureValue := "insecure-value"
	environment.EXPECT().
		GetHierarchicalValue(profileName, privateAPIKey).
		Return(nil)
	mockSecure.EXPECT().
		Get(profileName, privateAPIKey).
		Return("")
	insecure.EXPECT().
		GetHierarchicalValue(profileName, privateAPIKey).
		Return(insecureValue)

	result = store.GetHierarchicalValue(profileName, privateAPIKey)
	assert.Equal(t, insecureValue, result)
}

// TestGetGlobalValue_EnvVarPriority tests that environment variables
// take precedence over both secure store and insecure config for secure properties.
func TestGetGlobalValue_EnvVarPriority(t *testing.T) {
	ctrl := gomock.NewController(t)

	environment := mocks.NewMockStore(ctrl)
	insecure := mocks.NewMockStore(ctrl)
	mockSecure := mocks.NewMockSecureStore(ctrl)

	store := &ProxyStore{
		environment: environment,
		insecure:    insecure,
		secure:      mockSecure,
	}

	// Test that env var value is returned (no secure or insecure store calls)
	environment.EXPECT().
		GetGlobalValue(ClientIDField).
		Return(envVarTestValue)

	result := store.GetGlobalValue(ClientIDField)
	assert.Equal(t, envVarTestValue, result)
}
