# Atlas CLI Core

Atlas CLI Core is a Go library that provides the foundational configuration and profile management functionality for developing MongoDB Atlas CLI plugins. It serves as the core component for handling authentication, configuration management, and profile operations in Atlas CLI plugin development.

## What is Atlas CLI Core?

Atlas CLI Core is a shared library that plugin developers can use to integrate with the Atlas CLI's configuration system. It provides:

- **Shared Configuration Management**: Access to Atlas CLI's profile and configuration system
- **Profile Operations**: Multi-profile support with configuration file
- **Service Integration**: Built-in support for Atlas Cloud and Cloud Gov services

## Atlas CLI Plugins

Atlas CLI plugins extend the functionality of the MongoDB Atlas CLI with custom commands. A plugin consists of:

1. **Manifest File** (`manifest.yml`) - Defines the plugin's commands and metadata
2. **Executable Binary** - Contains the plugin's functionality
3. **Optional Assets** - Additional files like documentation or resources

When a plugin command is invoked, the Atlas CLI:
- Executes the plugin's binary
- Forwards all arguments, environment variables, and standard I/O streams
- Provides access to the Atlas CLI's configuration system through this library

## Features

- **Profile Management**: Multi-profile support with configuration file
- **Configuration**: Flexible configuration management with environment variable support
- **Service Support**: Built-in support for Atlas Cloud and Cloud Gov services

## Installation

### Prerequisites

- Go 1.22.5 or later
- Git

### As a Dependency

Add to your plugin's `go.mod`:

```go
require github.com/mongodb/atlas-cli-core v0.0.0
```

Then run:
```bash
go mod download
```

## Usage in Plugins

### Basic Plugin Setup

```go
package main

import (
    "fmt"
    "os"

    "github.com/mongodb/atlas-cli-core/config"
    "github.com/spf13/cobra"
)

func main() {
    // Initialize Atlas CLI configuration
    err := config.InitProfile("")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
        os.Exit(1)
    }

    // Create your plugin's root command
    rootCmd := &cobra.Command{
        Use:   "my-plugin",
        Short: "A custom Atlas CLI plugin",
        Long:  "This plugin extends Atlas CLI with custom functionality",
    }

    // Add your plugin commands here
    rootCmd.AddCommand(createExampleCmd())

    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

func createExampleCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "example",
        Short: "Example plugin command",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Access Atlas CLI configuration
            projectID := config.ProjectID()
            orgID := config.OrgID()
            
            fmt.Printf("Current project: %s\n", projectID)
            fmt.Printf("Current organization: %s\n", orgID)
            
            return nil
        },
    }
    return cmd
}
```

### Accessing Atlas CLI Configuration

```go
// Get current configuration values
projectID := config.ProjectID()
orgID := config.OrgID()
publicKey := config.PublicAPIKey()
privateKey := config.PrivateAPIKey()

// Check authentication status
if config.IsAccessSet() {
    // Get authentication token
    token, err := config.Token()
    if err != nil {
        return fmt.Errorf("failed to get token: %w", err)
    }
    
    // Use token for API calls
    fmt.Printf("Access Token: %s\n", token.AccessToken)
}

// Get current profile
profileName := config.Name()
fmt.Printf("Using profile: %s\n", profileName)
```

### Profile Management

```go
// List available profiles
profiles := config.List()
fmt.Println("Available profiles:", profiles)

// Switch to a different profile
err := config.SetName("production")
if err != nil {
    return fmt.Errorf("failed to switch profile: %w", err)
}

// Check if profile exists
exists := config.Exists("my-profile")
```

### Service Configuration

```go
// Check current service type
if config.IsCloud() {
    fmt.Println("Using Atlas Cloud service")
} else {
    fmt.Println("Using Atlas Cloud Gov service")
}

// Get service URL
baseURL := config.HttpBaseURL()
fmt.Printf("Service URL: %s\n", baseURL)
```

## Plugin Development

### Creating a Plugin

1. **Use the Template**: Start with the [Atlas CLI Plugin Example](https://github.com/mongodb/atlas-cli-plugin-example) as a template
2. **Add Dependencies**: Include `atlas-cli-core` in your `go.mod`
3. **Initialize Configuration**: Call `config.InitProfile("")` in your main function
4. **Access Configuration**: Use the library's functions to access Atlas CLI settings

### Manifest File

Your plugin's `manifest.yml` should look like this:

```yaml
name: your-plugin-name
description: Description of your plugin
version: 1.0.0
github:
    owner: your-github-username
    name: your-plugin-repo
binary: your-binary-name
commands:
    your-command:
        description: Description of your command
        aliases:
            - alias1
            - alias2
```

### Building and Releasing

1. **Build your plugin**:
   ```bash
   go build -o your-binary-name ./cmd/plugin
   ```

2. **Create a release** with proper assets:
   - Binary for each target platform
   - `manifest.yml` file
   - Any additional assets

3. **Install in Atlas CLI**:
   ```bash
   atlas plugin install your-github-username/your-plugin-repo
   ```

## Configuration

### Environment Variables

The library supports the following environment variable prefixes:

- `MCLI_` - Legacy MongoCLI prefix
- `MONGODB_ATLAS_` - Atlas CLI prefix

### Configuration Properties

| Property | Description | Type |
|----------|-------------|------|
| `project_id` | Atlas Project ID | string |
| `org_id` | Atlas Organization ID | string |
| `public_api_key` | Atlas Public API Key | string |
| `private_api_key` | Atlas Private API Key | string |
| `access_token` | OAuth Access Token | string |
| `refresh_token` | OAuth Refresh Token | string |
| `service` | Service type (cloud/cloudgov) | string |
| `output` | Output format | string |
| `telemetry_enabled` | Enable telemetry | boolean |
| `skip_update_check` | Skip update checks | boolean |

### Configuration File Location

Configuration files are stored in:
- **Linux/macOS**: `~/.config/atlascli/`
- **Windows**: `%APPDATA%\atlascli\`

## Development

### Building

```bash
# Build the library
go build ./...

# Run tests
make unit-test

# Run tests with coverage
go test -race -cover -count=1 -coverprofile coverage.out ./...
```

### Testing

```bash
# Run unit tests
make unit-test

# Run specific test file
go test ./config -v

# Run tests with tags
go test --tags="unit" ./...
```

### Code Generation

```bash
# Generate mocks
go generate ./...
```

## Project Structure

```
atlas-cli-core/
├── config/           # Configuration and profile management
│   ├── config.go     # Core configuration functionality
│   ├── profile.go    # Profile management and authentication
│   └── *_test.go     # Test files
├── mocks/            # Generated mock files
├── go.mod           # Go module definition
├── go.sum           # Go module checksums
├── Makefile         # Build and test commands
├── LICENSE          # Apache 2.0 License
└── README.md        # This file
```

## Examples

- [Atlas CLI Plugin Example](https://github.com/mongodb/atlas-cli-plugin-example) - Official plugin template
- [MongoDB Atlas CLI](https://github.com/mongodb/atlas-cli) - Main Atlas CLI tool

## Contributing

We welcome contributions! Please see our [CONTRIBUTING.md](CONTRIBUTING.md) file for details on how to submit pull requests, report issues, and contribute to the project.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: [MongoDB Atlas Documentation](https://docs.atlas.mongodb.com/)
- **Plugin Development**: [Atlas CLI Plugin Example](https://github.com/mongodb/atlas-cli-plugin-example)
- **Issues**: [GitHub Issues](https://github.com/mongodb/atlas-cli-core/issues)

## Related Projects

- [MongoDB Atlas CLI](https://github.com/mongodb/mongodb-atlas-cli) - Official MongoDB Atlas CLI tool
- [Atlas CLI Plugin Example](https://github.com/mongodb/atlas-cli-plugin-example) - Template for creating plugins
- [Atlas GO SDK](https://github.com/mongodb/atlas-sdk-go) - Official MongoDB Atlas GO SDK
