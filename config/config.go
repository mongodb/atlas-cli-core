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
	"bytes"
	"fmt"
	"os"
	"path"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

//go:generate mockgen -destination=../mocks/mock_profile.go -package=mocks github.com/mongodb/atlas-cli-core/config SetSaver

const (
	MongoCLIEnvPrefix        = "MCLI"          // MongoCLIEnvPrefix prefix for MongoCLI ENV variables
	AtlasCLIEnvPrefix        = "MONGODB_ATLAS" // AtlasCLIEnvPrefix prefix for AtlasCLI ENV variables
	DefaultProfile           = "default"       // DefaultProfile default
	CloudService             = "cloud"         // CloudService setting when using Atlas API
	CloudGovService          = "cloudgov"      // CloudGovService setting when using Atlas API for Government
	projectID                = "project_id"
	orgID                    = "org_id"
	mongoShellPath           = "mongosh_path"
	configType               = "toml"
	service                  = "service"
	publicAPIKey             = "public_api_key"
	privateAPIKey            = "private_api_key"
	AccessTokenField         = "access_token"
	RefreshTokenField        = "refresh_token"
	ClientIDField            = "client_id"
	OpsManagerURLField       = "ops_manager_url"
	baseURL                  = "base_url"
	output                   = "output"
	fileFlags                = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	configPerm               = 0600
	defaultPermissions       = 0700
	skipUpdateCheck          = "skip_update_check"
	TelemetryEnabledProperty = "telemetry_enabled"
	AtlasCLI                 = "atlascli"
	ContainerizedHostNameEnv = "MONGODB_ATLAS_IS_CONTAINERIZED"
	GitHubActionsHostNameEnv = "GITHUB_ACTIONS"
	AtlasActionHostNameEnv   = "ATLAS_GITHUB_ACTION"
	CLIUserTypeEnv           = "CLI_USER_TYPE" // CLIUserTypeEnv is used to separate MongoDB University users from default users
	DefaultUser              = "default"       // Users that do NOT use ATLAS CLI with MongoDB University
	UniversityUser           = "university"    // Users that uses ATLAS CLI with MongoDB University
	NativeHostName           = "native"
	DockerContainerHostName  = "container"
	GitHubActionsHostName    = "all_github_actions"
	AtlasActionHostName      = "atlascli_github_action"
)

var (
	HostName       = getConfigHostnameFromEnvs()
	CLIUserType    = newCLIUserTypeFromEnvs()
)

type Setter interface {
	Set(string, any)
}

type GlobalSetter interface {
	SetGlobal(string, any)
}

type Saver interface {
	Save() error
}

type SetSaver interface {
	Setter
	Saver
	GlobalSetter
}

func Properties() []string {
	return []string{
		projectID,
		orgID,
		service,
		publicAPIKey,
		privateAPIKey,
		output,
		OpsManagerURLField,
		baseURL,
		mongoShellPath,
		skipUpdateCheck,
		TelemetryEnabledProperty,
		AccessTokenField,
		RefreshTokenField,
	}
}

func BooleanProperties() []string {
	return []string{
		skipUpdateCheck,
		TelemetryEnabledProperty,
	}
}

func GlobalProperties() []string {
	return []string{
		skipUpdateCheck,
		TelemetryEnabledProperty,
		mongoShellPath,
	}
}

func IsTrue(s string) bool {
	switch strings.ToLower(s) {
	case "t", "true", "y", "yes", "1":
		return true
	default:
		return false
	}
}

func UserAgent(version string) string {
	return fmt.Sprintf("%s/%s (%s;%s;%s)", AtlasCLI, version, runtime.GOOS, runtime.GOARCH, HostName)
}

// List returns the names of available profiles.
func List() []string {
	m := viper.AllSettings()

	keys := make([]string, 0, len(m))
	for k := range m {
		if !slices.Contains(Properties(), k) {
			keys = append(keys, k)
		}
	}
	// keys in maps are non-deterministic, trying to give users a consistent output
	sort.Strings(keys)
	return keys
}

// Exists returns true if there are any set settings for the profile name.
func Exists(name string) bool {
	return slices.Contains(List(), name)
}

// getConfigHostnameFromEnvs patches the agent hostname based on set env vars.
func getConfigHostnameFromEnvs() string {
	var builder strings.Builder

	envVars := []struct {
		envName  string
		hostName string
	}{
		{AtlasActionHostNameEnv, AtlasActionHostName},
		{GitHubActionsHostNameEnv, GitHubActionsHostName},
		{ContainerizedHostNameEnv, DockerContainerHostName},
	}

	for _, envVar := range envVars {
		if envIsTrue(envVar.envName) {
			appendToHostName(&builder, envVar.hostName)
		} else {
			appendToHostName(&builder, "-")
		}
	}
	configHostName := builder.String()

	if isDefaultHostName(configHostName) {
		return NativeHostName
	}
	return configHostName
}

// newCLIUserTypeFromEnvs patches the user type information based on set env vars.
func newCLIUserTypeFromEnvs() string {
	if value, ok := os.LookupEnv(CLIUserTypeEnv); ok {
		return value
	}

	return DefaultUser
}

func envIsTrue(env string) bool {
	return IsTrue(os.Getenv(env))
}

func appendToHostName(builder *strings.Builder, configVal string) {
	if builder.Len() > 0 {
		builder.WriteString("|")
	}
	builder.WriteString(configVal)
}

// isDefaultHostName checks if the hostname is the default placeholder.
func isDefaultHostName(hostname string) bool {
	// Using strings.Count for a more dynamic approach.
	return strings.Count(hostname, "-") == strings.Count(hostname, "|")+1
}

func boolEnv(key string) bool {
	value, ok := os.LookupEnv(key)
	return ok && IsTrue(value)
}

func isTelemetryFeatureAllowed() bool {
	doNotTrack := boolEnv("DO_NOT_TRACK")
	return !doNotTrack
}

func hasMongoCLIEnvVars() bool {
	envVars := os.Environ()
	for _, v := range envVars {
		if strings.HasPrefix(v, MongoCLIEnvPrefix) {
			return true
		}
	}

	return false
}

// CLIConfigHome retrieves configHome path.
func CLIConfigHome() (string, error) {
	home, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return path.Join(home, "atlascli"), nil
}

func Path(f string) (string, error) {
	var p bytes.Buffer

	h, err := CLIConfigHome()
	if err != nil {
		return "", err
	}

	p.WriteString(h)
	p.WriteString(f)
	return p.String(), nil
}
