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
	"net/http"

	"github.com/mongodb/atlas-cli-core/config"
	"go.mongodb.org/atlas/auth"
)

func HTTPClient(version string, httpTransport http.RoundTripper) (*http.Client, error) {
	switch config.AuthType() {
	case config.APIKeys:
		t := NewDigestTransport(config.PublicAPIKey(), config.PrivateAPIKey(), httpTransport)
		return t.Client()
	case config.UserAccount:
		token, err := config.Token()
		if err != nil {
			return nil, err
		}

		// If the token is not nil, we're using the access token transport
		if token != nil {
			tr, err := NewAccessTokenTransport(token, httpTransport, version, func(t *auth.Token) error {
				config.SetAccessToken(t.AccessToken)
				config.SetRefreshToken(t.RefreshToken)
				return config.Save()
			})
			if err != nil {
				return nil, err
			}
			return &http.Client{Transport: tr}, nil
		}

		// No token available, we're falling back to the default client (default branch)
		fallthrough
	case config.ServiceAccount:
		return NewServiceAccountClientWithHost(config.ClientID(), config.ClientSecret(), config.OpsManagerURL()), nil
	case config.NoAuth:
		fallthrough
	default:
		return &http.Client{Transport: httpTransport}, nil
	}
}
