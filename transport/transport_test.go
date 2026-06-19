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
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mongodb/atlas-cli-core/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/atlas/auth"
)

func TestNewAccessTokenTransport(t *testing.T) {
	mockToken := &auth.Token{
		AccessToken:  "mock-access-token",
		RefreshToken: "mock-refresh-token",
	}

	saveToken := func(_ *auth.Token) error { return nil }

	base := Default()
	accessTokenTransport, err := NewAccessTokenTransport(mockToken, base, "1.0.0", saveToken)
	require.NoError(t, err)
	require.NotNil(t, accessTokenTransport)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com", nil)
	resp, err := accessTokenTransport.RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, resp)

	authHeader := req.Header.Get("Authorization")
	expectedHeader := "Bearer " + mockToken.AccessToken
	require.Equal(t, expectedHeader, authHeader)
}

func TestNewServiceAccountTransport(t *testing.T) {
	// Mock the token endpoint since the actual endpoint requires a valid client ID and secret.
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"access_token":"mock-token","token_type":"bearer","expires_in":3600}`)); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer tokenServer.Close()

	// Temporarily set OpsManagerURL to mock tokenServer URL
	originalURL := config.OpsManagerURL()
	config.SetOpsManagerURL(tokenServer.URL + "/")
	defer func() { config.SetOpsManagerURL(originalURL) }()

	clientID := "mock-client-id"
	clientSecret := "mock-client-secret" //nolint:gosec

	client := NewServiceAccountClientWithHost(clientID, clientSecret, tokenServer.URL)
	require.NotNil(t, client)

	// Create request to check authentication header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer mock-token" {
			t.Errorf("Expected Authorization header to be 'Bearer mock-token', but got: %v", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	resp, err := client.Transport.RoundTrip(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestDefaultTransport(t *testing.T) {
	transport := Default()
	require.NotNil(t, transport)

	assert.Zero(t, transport.TLSHandshakeTimeout, "TLSHandshakeTimeout should not be set for default transport")
	assert.Zero(t, transport.ResponseHeaderTimeout, "ResponseHeaderTimeout should not be set for default transport")
	assert.Equal(t, maxIdleConns, transport.MaxIdleConns)
	assert.Equal(t, maxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	assert.Equal(t, idleConnTimeout, transport.IdleConnTimeout)
	assert.Equal(t, expectContinueTimeout, transport.ExpectContinueTimeout)
}

func TestTelemetryTransport(t *testing.T) {
	transport := Telemetry()
	require.NotNil(t, transport)

	assert.Equal(t, telemetryTimeout, transport.TLSHandshakeTimeout, "TLSHandshakeTimeout should match telemetryTimeout constant")
	assert.Equal(t, telemetryTimeout, transport.ResponseHeaderTimeout, "ResponseHeaderTimeout should match telemetryTimeout constant")
	assert.Equal(t, maxIdleConns, transport.MaxIdleConns)
	assert.Equal(t, maxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	assert.Equal(t, idleConnTimeout, transport.IdleConnTimeout)
	assert.Equal(t, expectContinueTimeout, transport.ExpectContinueTimeout)
}

func TestNewTransport(t *testing.T) {
	transport := newTransport()
	require.NotNil(t, transport)

	// newTransport creates the default transport without strict timeouts
	assert.Zero(t, transport.TLSHandshakeTimeout, "TLSHandshakeTimeout should not be set")
	assert.Zero(t, transport.ResponseHeaderTimeout, "ResponseHeaderTimeout should not be set")
	assert.NotNil(t, transport.DialContext, "DialContext should be set")
}

func TestNewTelemetryTransport(t *testing.T) {
	transport := newTelemetryTransport()
	require.NotNil(t, transport)

	// newTelemetryTransport creates transport with strict timeouts
	assert.Equal(t, telemetryTimeout, transport.TLSHandshakeTimeout, "TLSHandshakeTimeout should match telemetryTimeout")
	assert.Equal(t, telemetryTimeout, transport.ResponseHeaderTimeout, "ResponseHeaderTimeout should match telemetryTimeout")
	assert.NotNil(t, transport.DialContext, "DialContext should be set")
}

func TestRequestTimeout_TimesOut(t *testing.T) {
	// Create a server that delays sending response headers longer than the timeout
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Sleep longer than our context timeout
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	transport := newTransport()

	client := &http.Client{
		Transport: transport,
	}

	// Use context with short timeout to simulate request timeout
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, slowServer.URL, nil)
	require.NoError(t, err)

	start := time.Now()
	_, err = client.Do(req)
	elapsed := time.Since(start)

	// Should have timed out
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded",
		"expected context deadline exceeded error, got: %v", err)

	// Should have timed out close to our context timeout, not waited for full server delay
	assert.Less(t, elapsed, 300*time.Millisecond,
		"request should have timed out around 100ms, but took %v", elapsed)
}

func TestResponseHeaderTimeout_SucceedsWithinTimeout(t *testing.T) {
	// Create a server that responds quickly
	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer fastServer.Close()

	// Use telemetry transport (has strict timeouts)
	transport := newTelemetryTransport()

	client := &http.Client{
		Transport: transport,
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, fastServer.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTelemetryTimeout_SlowServerDoesNotBlock(t *testing.T) {
	// Simulate a server that takes way too long (like the telemetry issue)
	verySlowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Simulate a 3 second delay (longer than telemetry timeout)
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer verySlowServer.Close()

	// Use the telemetry transport with strict timeouts
	transport := newTelemetryTransport()

	// Use http.Client.Timeout for overall request timeout (this is what should be used in production)
	client := &http.Client{
		Transport: transport,
		Timeout:   telemetryTimeout,
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, verySlowServer.URL, nil)
	require.NoError(t, err)

	start := time.Now()
	_, err = client.Do(req)
	elapsed := time.Since(start)

	// Should have timed out
	require.Error(t, err)
	assert.True(t,
		strings.Contains(err.Error(), "Client.Timeout") || strings.Contains(err.Error(), "context deadline exceeded"),
		"error should mention timeout, got: %v", err)

	// Should timeout around 1 second, definitely not wait for the full 3 seconds
	assert.Less(t, elapsed, 2*time.Second,
		"telemetry request should timeout around 1s, but took %v", elapsed)
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond,
		"request should have waited close to the timeout before giving up")
}
