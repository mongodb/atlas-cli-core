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

package transport

import (
	"context"
	"net/http"
	"time"

	"github.com/mongodb/atlas-cli-core/config"
	"go.mongodb.org/atlas/auth"
	"golang.org/x/oauth2"
)

//go:generate go tool go.uber.org/mock/mockgen -destination=mock_profile_provider.go -package=transport github.com/mongodb/atlas-cli-core/transport ProfileProvider

// ProfileProvider defines the interface for profile operations needed by HTTP client creation.
type ProfileProvider interface {
	AuthType() config.AuthMechanism
	PublicAPIKey() string
	PrivateAPIKey() string
	Token() (*auth.Token, error)
	ServiceAccountToken() (*auth.Token, error)
	SetAccessToken(string)
	SetRefreshToken(string)
	SetTokenExpiry(string)
	Save() error
	ClientID() string
	ClientSecret() string
	OpsManagerURL() string
	AuthServerMetadata() map[string]any
}

func HTTPClient(version string, httpTransport http.RoundTripper) (*http.Client, error) {
	return HTTPClientFromProfile(config.Default(), version, httpTransport)
}

func HTTPClientFromProfile(profile ProfileProvider, version string, httpTransport http.RoundTripper) (*http.Client, error) {
	switch profile.AuthType() {
	case config.APIKeys:
		t := NewDigestTransport(profile.PublicAPIKey(), profile.PrivateAPIKey(), httpTransport)
		return t.Client()
	case config.UserAccount:
		token, err := profile.Token()
		if err != nil {
			return nil, err
		}

		// If the token is not nil, we're using the access token transport
		if token != nil {
			tr, err := NewAccessTokenTransport(token, httpTransport, version, func(t *auth.Token) error {
				profile.SetAccessToken(t.AccessToken)
				profile.SetRefreshToken(t.RefreshToken)
				return profile.Save()
			})
			if err != nil {
				return nil, err
			}
			return &http.Client{Transport: tr}, nil
		}

		// No token available, we're falling back to the default client (default branch)
		fallthrough
	case config.ServiceAccount:
		seed, err := profile.ServiceAccountToken()
		if err != nil {
			return nil, err
		}
		return NewServiceAccountClientWithHost(context.Background(), profile.ClientID(), profile.ClientSecret(), profile.OpsManagerURL(), version, seed,
			func(t *oauth2.Token) error {
				profile.SetAccessToken(t.AccessToken)
				return profile.Save()
			}), nil
	case config.UserDelegation:
		token, err := profile.Token()
		if err != nil {
			return nil, err
		}

		if token != nil {
			tr, err := NewAccessTokenTransportForAuthIssuer(token, httpTransport, version, profile.AuthServerMetadata(), func(t *auth.Token) error {
				profile.SetAccessToken(t.AccessToken)
				profile.SetRefreshToken(t.RefreshToken)
				if !t.Expiry.IsZero() {
					profile.SetTokenExpiry(t.Expiry.Format(time.RFC3339))
				}
				return profile.Save()
			})
			if err != nil {
				return nil, err
			}
			return &http.Client{Transport: tr}, nil
		}

		// No token available, fall through to unauthenticated client
		fallthrough
	case config.NoAuth:
		fallthrough
	default:
		return &http.Client{Transport: httpTransport}, nil
	}
}
