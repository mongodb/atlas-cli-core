// Copyright 2025 MongoDB Inc
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

package transport

import (
	"net/http"
	"os"

	"github.com/mongodb/atlas-cli-core/config"
	"go.mongodb.org/atlas-sdk/v20250312027/auth/code"
	"go.mongodb.org/atlas/auth"
)

const (
	cloudGovServiceURL = "https://cloud.mongodbgov.com/"

	// Client IDs
	ClientID    = "0oabtxactgS3gHIR0297" // ClientID for production
	GovClientID = "0oabtyfelbTBdoucy297" // GovClientID for production

	authServerURLEnvVar = "MONGODB_ATLAS_AUTH_SERVER_URL"
	// authCodeClientID is the registered public client ID, used across environments.
	// MONGODB_ATLAS_CLIENT_ID is deliberately not honored: it is the service-account client ID variable.
	authCodeClientID = "https://mongodb.com/products/tools/atlas-cli"
	// govAuthServerURL is the Authorization Server issuer for the Government service.
	govAuthServerURL = "https://authorize.mongodbgov.com"
)

type ServiceGetter interface {
	Service() string
	OpsManagerURL() string
	ClientID() string
	AccountURL() string
}

func FlowWithConfig(c ServiceGetter, client *http.Client, version string) (*auth.Config, error) {
	id := ClientID
	if c.Service() == config.CloudGovService {
		id = GovClientID
	}
	if c.ClientID() != "" {
		id = c.ClientID()
	}

	authOpts := []auth.ConfigOpt{
		auth.SetUserAgent(config.UserAgent(version)),
		auth.SetClientID(id),
		auth.SetScopes([]string{"openid", "profile", "offline_access"}),
	}

	if configURL := c.AccountURL(); configURL != "" {
		authOpts = append(authOpts, auth.SetAuthURL(configURL))
	} else if configURL := c.OpsManagerURL(); configURL != "" {
		authOpts = append(authOpts, auth.SetAuthURL(c.OpsManagerURL()))
	} else if c.Service() == config.CloudGovService {
		authOpts = append(authOpts, auth.SetAuthURL(cloudGovServiceURL))
	}
	return auth.NewConfigWithOptions(client, authOpts...)
}

// AuthServerGetter provides the profile settings needed for the Authorization
// Code + PKCE flow.
type AuthServerGetter interface {
	Service() string
	ClientID() string
}

// FlowForAuthServer creates the Authorization Code + PKCE config for the dedicated
// Authorization Server. It intentionally does not read OpsManagerURL or AccountURL;
// the issuer comes from MONGODB_ATLAS_AUTH_SERVER_URL, defaulting per service.
func FlowForAuthServer(c AuthServerGetter, client *http.Client, version string) (*code.Config, error) {
	id := authCodeClientID
	if c.ClientID() != "" {
		id = c.ClientID()
	}

	authOpts := []code.ConfigOpt{
		code.SetUserAgent(config.UserAgent(version)),
		code.SetClientID(id),
		code.SetScopes([]string{"atlas"}),
	}
	// The SDK applies MONGODB_ATLAS_AUTH_SERVER_URL by default; only default the
	// gov issuer when the environment variable is unset.
	if os.Getenv(authServerURLEnvVar) == "" && c.Service() == config.CloudGovService {
		authOpts = append(authOpts, code.SetAuthServerURL(govAuthServerURL))
	}

	return code.NewConfigWithOptions(client, authOpts...)
}
