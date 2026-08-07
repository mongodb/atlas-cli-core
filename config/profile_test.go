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
	"context"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/mongodb/atlas-cli-core/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCLIConfigHome(t *testing.T) {
	expHome, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("os.UserConfigDir() unexpected error: %v", err)
	}
	home, err := CLIConfigHome()
	if err != nil {
		t.Fatalf("AtlasCLIConfigHome() unexpected error: %v", err)
	}
	expected := path.Join(expHome, "atlascli")
	if home != expected {
		t.Errorf("AtlasCLIConfigHome() = %s; want '%s'", home, expected)
	}
}

func TestConfig_IsTrue(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{
			input: "true",
			want:  true,
		},
		{
			input: "True",
			want:  true,
		},
		{
			input: "t",
			want:  true,
		},
		{
			input: "T",
			want:  true,
		},
		{
			input: "TRUE",
			want:  true,
		},
		{
			input: "y",
			want:  true,
		},
		{
			input: "Y",
			want:  true,
		},
		{
			input: "yes",
			want:  true,
		},
		{
			input: "Yes",
			want:  true,
		},
		{
			input: "YES",
			want:  true,
		},
		{
			input: "1",
			want:  true,
		},
		{
			input: "false",
			want:  false,
		},
		{
			input: "f",
			want:  false,
		},
		{
			input: "unknown",
			want:  false,
		},
		{
			input: "0",
			want:  false,
		},
		{
			input: "",
			want:  false,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			if got := IsTrue(tt.input); got != tt.want {
				t.Errorf("IsTrue() get: %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getConfigHostname(t *testing.T) {
	type fields struct {
		containerizedEnv string
		atlasActionEnv   string
		ghActionsEnv     string
	}
	tests := []struct {
		name             string
		fields           fields
		expectedHostName string
	}{
		{
			name: "sets native hostname when no hostname env var is set",
			fields: fields{
				containerizedEnv: "",
				atlasActionEnv:   "",
				ghActionsEnv:     "",
			},
			expectedHostName: NativeHostName,
		},
		{
			name: "sets container hostname when containerized env var is set",
			fields: fields{
				containerizedEnv: "true",
				atlasActionEnv:   "",
				ghActionsEnv:     "",
			},
			expectedHostName: "-|-|" + DockerContainerHostName,
		},
		{
			name: "sets atlas action hostname when containerized env var is set",
			fields: fields{
				containerizedEnv: "",
				atlasActionEnv:   "true",
				ghActionsEnv:     "",
			},
			expectedHostName: AtlasActionHostName + "|-|-",
		},
		{
			name: "sets github actions hostname when action env var is set",
			fields: fields{
				containerizedEnv: "",
				atlasActionEnv:   "",
				ghActionsEnv:     "true",
			},
			expectedHostName: "-|" + GitHubActionsHostName + "|-",
		},
		{
			name: "sets actions and containerized hostnames when both env vars are set",
			fields: fields{
				containerizedEnv: "true",
				atlasActionEnv:   "true",
				ghActionsEnv:     "true",
			},
			expectedHostName: AtlasActionHostName + "|" + GitHubActionsHostName + "|" + DockerContainerHostName,
		},
	}
	for _, tt := range tests {
		f := tt.fields
		expectedHostName := tt.expectedHostName
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(AtlasActionHostNameEnv, f.atlasActionEnv)
			t.Setenv(GitHubActionsHostNameEnv, f.ghActionsEnv)
			t.Setenv(ContainerizedHostNameEnv, f.containerizedEnv)
			actualHostName := getConfigHostnameFromEnvs()

			assert.Equal(t, expectedHostName, actualHostName)
		})
	}
}

func TestProfile_Rename(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "default",
			wantErr: false,
		},
		{
			name:    "default-123",
			wantErr: false,
		},
		{
			name:    "default-test",
			wantErr: false,
		},
		{
			name:    "default.123",
			wantErr: true,
		},
		{
			name:    "default.test",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var assertion require.ErrorAssertionFunc
			if tt.wantErr {
				assertion = require.Error
			} else {
				assertion = require.NoError
			}

			ctrl := gomock.NewController(t)
			configStore := mocks.NewMockStore(ctrl)
			if !tt.wantErr {
				configStore.EXPECT().RenameProfile(DefaultProfile, tt.name).Return(nil).Times(1)
			}

			p := &Profile{
				name:        DefaultProfile,
				configStore: configStore,
			}

			assertion(t, p.Rename(tt.name), fmt.Sprintf("Rename(%v)", tt.name))
		})
	}
}

func TestProfile_SetName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr require.ErrorAssertionFunc
	}{
		{
			name:    "default",
			wantErr: require.NoError,
		},
		{
			name:    "default-123",
			wantErr: require.NoError,
		},
		{
			name:    "default-test",
			wantErr: require.NoError,
		},
		{
			name:    "default.123",
			wantErr: require.Error,
		},
		{
			name:    "default.test",
			wantErr: require.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			p := &Profile{
				name:        tt.name,
				configStore: mocks.NewMockStore(ctrl),
			}
			tt.wantErr(t, p.SetName(tt.name), fmt.Sprintf("SetName(%v)", tt.name))
		})
	}
}

func TestWithProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile *Profile
		ctx     context.Context
	}{
		{
			name:    "add profile to empty context",
			profile: &Profile{name: "test-profile"},
			ctx:     t.Context(),
		},
		{
			name:    "add profile to context with existing values",
			profile: &Profile{name: "another-profile"},
			ctx:     WithProfile(t.Context(), &Profile{name: "test-profile"}),
		},
		{
			name:    "add nil profile",
			profile: nil,
			ctx:     t.Context(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WithProfile(tt.ctx, tt.profile)

			// Verify context is not nil
			require.NotNil(t, result)

			// Verify the profile was stored correctly
			storedProfile, ok := result.Value(profileContextKey).(*Profile)
			if tt.profile == nil {
				assert.Nil(t, storedProfile)
			} else {
				require.True(t, ok)
				assert.Equal(t, tt.profile, storedProfile)
				assert.Equal(t, tt.profile.name, storedProfile.name)
			}

			// Verify original context values are preserved
			if tt.ctx != t.Context() {
				if existingValue := result.Value("existing-key"); existingValue != nil {
					assert.Equal(t, "existing-value", existingValue)
				}
			}
		})
	}
}

func TestProfileFromContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := mocks.NewMockStore(ctrl)

	tests := []struct {
		name            string
		ctx             context.Context
		expectedProfile *Profile
		expectedOk      bool
	}{
		{
			name:            "retrieve profile from context",
			ctx:             WithProfile(t.Context(), &Profile{name: "test-profile", configStore: mockStore}),
			expectedProfile: &Profile{name: "test-profile", configStore: mockStore},
			expectedOk:      true,
		},
		{
			name:            "no profile in context",
			ctx:             t.Context(),
			expectedProfile: nil,
			expectedOk:      false,
		},
		{
			name:            "nil context",
			ctx:             nil,
			expectedProfile: nil,
			expectedOk:      false,
		},
		{
			name:            "context with nil profile",
			ctx:             WithProfile(t.Context(), nil),
			expectedProfile: nil,
			expectedOk:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, ok := ProfileFromContext(tt.ctx)

			assert.Equal(t, tt.expectedOk, ok)

			if tt.expectedProfile == nil {
				assert.Nil(t, profile)
			} else {
				require.NotNil(t, profile)
				assert.Equal(t, tt.expectedProfile.name, profile.name)
				assert.Equal(t, tt.expectedProfile.configStore, profile.configStore)
			}
		})
	}
}

func TestWithProfile_ProfileFromContext_RoundTrip(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := mocks.NewMockStore(ctrl)

	// Create a test profile with some data
	originalProfile := NewProfile("test-profile", mockStore)

	// Store it in context
	ctx := WithProfile(t.Context(), originalProfile)

	// Retrieve it back
	retrievedProfile, ok := ProfileFromContext(ctx)

	// Verify round trip worked
	require.True(t, ok)
	require.NotNil(t, retrievedProfile)
	assert.Equal(t, originalProfile.name, retrievedProfile.name)
	assert.Equal(t, originalProfile.configStore, retrievedProfile.configStore)
	assert.Same(t, originalProfile, retrievedProfile) // Should be the exact same object
}

func TestWithProfile_Multiple_Profiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore1 := mocks.NewMockStore(ctrl)
	mockStore2 := mocks.NewMockStore(ctrl)

	// Create multiple profiles
	profile1 := NewProfile("profile1", mockStore1)
	profile2 := NewProfile("profile2", mockStore2)

	// Add first profile to context
	ctx1 := WithProfile(t.Context(), profile1)

	// Verify first profile is stored
	retrieved1, ok := ProfileFromContext(ctx1)
	require.True(t, ok)
	assert.Equal(t, "profile1", retrieved1.name)

	// Override with second profile
	ctx2 := WithProfile(ctx1, profile2)

	// Verify second profile overwrites the first
	retrieved2, ok := ProfileFromContext(ctx2)
	require.True(t, ok)
	assert.Equal(t, "profile2", retrieved2.name)

	// Verify original context still has first profile
	stillRetrieved1, ok := ProfileFromContext(ctx1)
	require.True(t, ok)
	assert.Equal(t, "profile1", stillRetrieved1.name)
}

// jwtWithExpiry builds an unsigned-verification-safe JWT string carrying the given expiry.
func jwtWithExpiry(t *testing.T, exp time.Time) string {
	t.Helper()
	claims := jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(exp)}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	require.NoError(t, err)
	return s
}

func TestProfile_ServiceAccountToken(t *testing.T) {
	t.Run("returns nil when no access token is persisted", func(t *testing.T) {
		p := NewProfile("test", NewInMemoryStore())
		tok, err := p.ServiceAccountToken()
		require.NoError(t, err)
		assert.Nil(t, tok)
	})

	t.Run("returns nil when the persisted access token is not a JWT", func(t *testing.T) {
		p := NewProfile("test", NewInMemoryStore())
		p.SetAccessToken("not-a-jwt")
		tok, err := p.ServiceAccountToken()
		require.NoError(t, err)
		assert.Nil(t, tok, "a non-JWT token should be treated as no token so a fresh one is minted")
	})

	t.Run("returns token with expiry parsed from the JWT and no refresh token", func(t *testing.T) {
		exp := time.Now().Add(45 * time.Minute).Truncate(time.Second)
		p := NewProfile("test", NewInMemoryStore())
		p.SetAccessToken(jwtWithExpiry(t, exp))

		tok, err := p.ServiceAccountToken()
		require.NoError(t, err)
		require.NotNil(t, tok)
		assert.Equal(t, p.AccessToken(), tok.AccessToken)
		assert.Equal(t, "Bearer", tok.TokenType)
		assert.Empty(t, tok.RefreshToken, "service account tokens have no refresh token")
		assert.WithinDuration(t, exp, tok.Expiry, time.Second)
	})
}

func TestProfile_TokenExpiry(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := mocks.NewMockStore(ctrl)
	p := &Profile{name: DefaultProfile, configStore: mockStore}

	mockStore.EXPECT().SetProfileValue(DefaultProfile, TokenExpiryField, "2026-05-13T12:00:00Z").Times(1)
	mockStore.EXPECT().GetHierarchicalValue(DefaultProfile, TokenExpiryField).Return("2026-05-13T12:00:00Z").Times(1)

	p.SetTokenExpiry("2026-05-13T12:00:00Z")
	assert.Equal(t, "2026-05-13T12:00:00Z", p.TokenExpiry())
}

func TestProfile_AuthServerURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := mocks.NewMockStore(ctrl)
	p := &Profile{name: DefaultProfile, configStore: mockStore}

	mockStore.EXPECT().SetProfileValue(DefaultProfile, AuthServerURLField, "https://authorize.example.com").Times(1)
	mockStore.EXPECT().GetHierarchicalValue(DefaultProfile, AuthServerURLField).Return("https://authorize.example.com").Times(1)

	p.SetAuthServerURL("https://authorize.example.com")
	assert.Equal(t, "https://authorize.example.com", p.AuthServerURL())

	// AuthServerURL is user-configurable, so it must appear in ProfileProperties()
	assert.Contains(t, ProfileProperties(), AuthServerURLField)
}

func TestProfile_AuthServerMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStore := mocks.NewMockStore(ctrl)
	p := &Profile{name: DefaultProfile, configStore: mockStore}

	metadata := map[string]any{
		"metadata": map[string]any{"issuer": "https://authorize.example.com"},
		"expiry":   "2026-05-20T00:00:00Z",
	}

	mockStore.EXPECT().SetProfileValue(DefaultProfile, authServerMetadataField, metadata).Times(1)
	p.SetAuthServerMetadata(metadata)

	t.Run("round-trip", func(t *testing.T) {
		mockStore.EXPECT().GetHierarchicalValue(DefaultProfile, authServerMetadataField).Return(metadata).Times(1)
		assert.Equal(t, metadata, p.AuthServerMetadata())
	})

	t.Run("nil when unset", func(t *testing.T) {
		mockStore.EXPECT().GetHierarchicalValue(DefaultProfile, authServerMetadataField).Return(nil).Times(1)
		assert.Nil(t, p.AuthServerMetadata())
	})

	t.Run("nil when underlying value is wrong type", func(t *testing.T) {
		mockStore.EXPECT().GetHierarchicalValue(DefaultProfile, authServerMetadataField).Return("not-a-map").Times(1)
		assert.Nil(t, p.AuthServerMetadata())
	})
}

func TestProfile_TokenClaims_UserDelegation(t *testing.T) {
	p := &Profile{name: DefaultProfile, configStore: NewInMemoryStore()}
	// AuthType()'s env-var sniffing reads from Default(), so route those calls
	// through the same profile for the duration of the test.
	prev := Default()
	SetDefaultProfile(p)
	t.Cleanup(func() { SetDefaultProfile(prev) })

	p.SetAuthType(UserDelegation)
	p.SetTokenExpiry("2026-05-13T12:00:00Z")

	claims, err := p.tokenClaims()
	require.NoError(t, err)
	require.NotNil(t, claims.ExpiresAt)
	assert.Equal(t, "2026-05-13T12:00:00Z", claims.ExpiresAt.Format("2006-01-02T15:04:05Z"))
	// Subject is not populated for UserDelegation — the access token is not parsed
	assert.Empty(t, claims.Subject)
}
