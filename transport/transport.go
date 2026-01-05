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
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mongodb-forks/digest"
	"github.com/mongodb/atlas-cli-core/config"
	"go.mongodb.org/atlas-sdk/v20250312006/auth/clientcredentials"
	atlasauth "go.mongodb.org/atlas/auth"
)

const (
	telemetryTimeout      = 1 * time.Second
	timeout               = 5 * time.Second
	keepAlive             = 30 * time.Second
	maxIdleConns          = 5
	maxIdleConnsPerHost   = 4
	idleConnTimeout       = 30 * time.Second
	expectContinueTimeout = 1 * time.Second
)

var defaultTransport = newTransport()

func Default() *http.Transport {
	return defaultTransport
}

var telemetryTransport = newTelemetryTransport()

func Telemetry() *http.Transport {
	return telemetryTransport
}

func newTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout,
			KeepAlive: keepAlive,
		}).DialContext,
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		Proxy:                 http.ProxyFromEnvironment,
		IdleConnTimeout:       idleConnTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
	}
}

// newTelemetryTransport creates a transport with strict timeouts for telemetry.
// Telemetry should be fire-and-forget and not block the CLI.
func newTelemetryTransport() *http.Transport {
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   telemetryTimeout,
			KeepAlive: keepAlive,
		}).DialContext,
		TLSHandshakeTimeout:   telemetryTimeout,
		ResponseHeaderTimeout: telemetryTimeout,
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		Proxy:                 http.ProxyFromEnvironment,
		IdleConnTimeout:       idleConnTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
	}
}

func NewDigestTransport(username, password string, base http.RoundTripper) *digest.Transport {
	return &digest.Transport{
		Username:  username,
		Password:  password,
		Transport: base,
	}
}

func NewAccessTokenTransport(token *atlasauth.Token, base http.RoundTripper, version string, saveToken func(*atlasauth.Token) error) (http.RoundTripper, error) {
	if token == nil {
		return nil, errors.New("token is nil")
	}

	client := http.DefaultClient
	client.Transport = Default()

	flow, err := FlowWithConfig(config.Default(), client, version)

	if err != nil {
		return nil, err
	}

	return &tokenTransport{
		token:      token,
		base:       base,
		authConfig: flow,
		saveToken:  saveToken,
	}, nil
}

type tokenTransport struct {
	token      *atlasauth.Token
	authConfig *atlasauth.Config
	base       http.RoundTripper
	saveToken  func(*atlasauth.Token) error
}

func (tr *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !tr.token.Valid() {
		token, _, err := tr.authConfig.RefreshToken(req.Context(), tr.token.RefreshToken)
		if err != nil {
			return nil, err
		}
		tr.token = token
		if err := tr.saveToken(tr.token); err != nil {
			return nil, err
		}
	}

	tr.token.SetAuthHeader(req)

	return tr.base.RoundTrip(req)
}

// NewServiceAccountClientWithHost creates a new HTTP client configured for service account authentication.
// This function does not return http.RoundTripper as atlas-sdk already packages a transport with the client.
func NewServiceAccountClientWithHost(clientID, clientSecret, host string) *http.Client {
	cfg := clientcredentials.NewConfig(clientID, clientSecret)
	if host != "" {
		baseURL := strings.TrimSuffix(host, "/")
		cfg.TokenURL = baseURL + clientcredentials.TokenAPIPath
		cfg.RevokeURL = baseURL + clientcredentials.RevokeAPIPath
	}
	return cfg.Client(context.Background())
}
