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
	"net/url"

	"github.com/mongodb/atlas-cli-core/config"
	"go.mongodb.org/atlas/auth"
)

const (
	cloudGovServiceURL = "https://cloud.mongodbgov.com/"

	// Client IDs for the existing auth path (cloud.mongodb.com proxy → auth.mongodb.com)
	ClientID    = "0oabtxactgS3gHIR0297" // ClientID for production
	GovClientID = "0oabtyfelbTBdoucy297" // GovClientID for production

	// Client ID and gov URL for the dedicated OAuth Authorization Server.
	// The commercial auth issuer URL comes from the auth.Config.AuthServerURL field,
	// populated by default in auth.NewConfig from go.mongodb.org/atlas.
	// The gov URL is a placeholder pending the gov AS deployment.
	govDefaultAuthIssuerURL = "https://authorize.mongodbgov.com"
	authIssuerClientID      = "5cc53c56-022d-41e7-928f-46621e62f8c1"
)

type ServiceGetter interface {
	Service() string
	OpsManagerURL() string
	ClientID() string
	AccountURL() string
}

// AuthIssuerGetter provides the config needed for the dedicated OAuth AS flow.
type AuthIssuerGetter interface {
	Service() string
	ClientID() string
	AuthServerURL() string
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

// FlowForAuthIssuer creates an auth.Config for the dedicated OAuth Authorization Server.
// This is the parallel path for UserDelegation sessions and intentionally does not read
// OpsManagerURL or AccountURL — the auth server URL comes from the Config's AuthServerURL
// field, populated by default in auth.NewConfig.
func FlowForAuthIssuer(c AuthIssuerGetter, client *http.Client, version string) (*auth.Config, error) {
	id := authIssuerClientID
	if c.ClientID() != "" {
		id = c.ClientID()
	}

	cfg, err := auth.NewConfigWithOptions(client,
		auth.SetUserAgent(config.UserAgent(version)),
		auth.SetClientID(id),
		auth.SetScopes([]string{"atlas"}),
	)
	if err != nil {
		return nil, err
	}

	if configURL := c.AuthServerURL(); configURL != "" {
		cfg.AuthServerURL, err = url.Parse(configURL)
		if err != nil {
			return nil, err
		}
	} else if c.Service() == config.CloudGovService {
		cfg.AuthServerURL, err = url.Parse(govDefaultAuthIssuerURL)
		if err != nil {
			return nil, err
		}
	}

	return cfg, nil
}
