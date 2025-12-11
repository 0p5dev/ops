# ops

> A developer-first CLI tool for deploying autoscaling applications with minimal configuration

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)

## Overview

`ops` is a command-line interface tool designed to streamline the deployment of containerized applications. It provides a simple, developer-friendly workflow for building, uploading, and deploying Docker containers to the 0p5.dev platform with automatic scaling capabilities.

### Key Features

- 🚀 **One-command deployment** - Build, compress, upload, and deploy in a single command
- 🔐 **OAuth-based authentication** - Secure login flow using Supabase Auth
- 🐳 **Docker integration** - Seamless container building and image transmission
- 📦 **Automatic compression** - Efficient image transfer with gzip compression
- ⚡ **Interactive CLI** - Beautiful prompts and real-time progress indicators
- 🔧 **Configurable** - Support for custom controller endpoints and environment-specific settings

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Commands](#commands)
- [Configuration](#configuration)
- [Development](#development)
- [Architecture](#architecture)
- [CI/CD](#cicd)
- [Contributing](#contributing)
- [License](#license)

## Installation

### From Release

Download the latest release from the [GitHub Releases page](https://github.com/0p5dev/ops/releases):

```bash
# Download and extract
wget https://github.com/0p5dev/ops/releases/latest/download/ops_linux_amd64.tgz
tar -xzf ops_linux_amd64.tgz

# Move to PATH
sudo mv ops_linux_amd64 /usr/local/bin/ops
sudo chmod +x /usr/local/bin/ops
```

### From Source

**Prerequisites:**
- Go 1.25 or higher
- Docker (for deployment functionality)
- [just](https://github.com/casey/just) command runner (optional, but recommended)

```bash
# Clone the repository
git clone https://github.com/0p5dev/ops.git
cd ops

# Install using just
just install

# Or install using go directly
go install
```

## Quick Start

### 1. Authenticate

Before deploying, you need to authenticate with 0p5.dev:

```bash
ops login
```

This will:
- Open your browser to complete OAuth authentication
- Store your access and refresh tokens securely in `~/.config/ops/tokens.json`
- Validate your credentials with the platform

### 2. Deploy Your Application

From your project directory containing a `Dockerfile`:

```bash
ops deploy
```

You'll be prompted for a deployment name, then `ops` will:
1. Build your Docker image
2. Compress the image to a `.tgz` file
3. Upload to the platform
4. Create and start your deployment
5. Return your service URL

Example output:
```
Deployment Name: my-awesome-app
✓ Container image built successfully
Saving and compressing container image... ━━━━━━━━━━━━━━━━━━
Uploading container image... ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✓ Creating deployment...
Deployment successful! Your service is available at: https://my-awesome-app.0p5.dev
```

## Commands

### `ops deploy` (alias: `d`)

Deploy a containerized application to the platform.

```bash
ops deploy
ops d  # shorthand
```

**Prerequisites:**
- A `Dockerfile` in your current directory
- Docker daemon running
- Valid authentication (run `ops login` first)

**Process:**
1. Prompts for deployment name
2. Builds Docker image with the deployment name as tag
3. Saves and compresses image to `<deployment-name>.tgz`
4. Uploads compressed image to controller
5. Creates deployment with free tier configuration
6. Returns service URL

**Environment Variables:**
- `CONTROLLER_BASE_URL` - Override the controller endpoint (default: `http://34.58.48.78`)

### `ops login` (alias: `l`)

Authenticate with the 0p5.dev platform using OAuth.

```bash
ops login
ops l  # shorthand
```

**Process:**
1. Starts local callback server on port 54321
2. Opens browser to Supabase OAuth login page
3. Handles OAuth redirect and token exchange
4. Stores access token, refresh token, and user info in `~/.config/ops/tokens.json`
5. Automatically refreshes expired tokens on subsequent commands

**Configuration:**
- Requires `SUPABASE_URL` and `SUPABASE_KEY` to be set at build time or via environment variables
- Tokens are stored securely with 600 permissions
- Refresh tokens are automatically used when access tokens expire

## Configuration

### User Configuration File

Create `~/.config/ops/config.json` to customize behavior:

```json
{
  "controllerBaseUrl": "http://custom-controller.example.com"
}
```

### Environment Variables

The following environment variables can override configuration:

| Variable | Description | Default |
|----------|-------------|---------|
| `SUPABASE_URL` | Supabase project URL | Set at build time |
| `SUPABASE_KEY` | Supabase anonymous key | Set at build time |
| `CONTROLLER_BASE_URL` | Platform controller endpoint | `http://34.58.48.78` |

### Build-time Configuration

When building from source, inject Supabase credentials:

```bash
go build -ldflags "\
  -X github.com/0p5dev/ops/internal/config.SupabaseURL=https://your-project.supabase.co \
  -X github.com/0p5dev/ops/internal/config.SupabaseKey=your-anon-key" \
  -o ops .
```

## Development

### Project Structure

```
ops/
├── main.go                    # CLI entry point and command definitions
├── internal/
│   ├── auth/
│   │   └── auth.go           # OAuth flow, token management, and refresh logic
│   ├── config/
│   │   └── config.go         # Configuration loading and defaults
│   ├── deploy/
│   │   └── main.go           # Deployment workflow and API interactions
│   ├── prompts/
│   │   └── main.go           # Interactive user prompts
│   └── ui/
│       ├── spinner.go        # Spinner animations for tasks
│       └── status.go         # Progress indicators
├── go.mod                     # Go module dependencies
├── justfile                   # Task automation with just
├── cloudbuild.yaml           # Google Cloud Build CI/CD pipeline
└── LICENSE                    # MIT License
```

### Development Commands

Using [just](https://github.com/casey/just):

```bash
# Format code
just fmt

# Run locally
just run

# Build binary
just build

# Install to GOPATH
just install

# Tidy dependencies
just tidy

# Add a dependency
just add github.com/some/package
```

Using Go directly:

```bash
# Run
go run main.go

# Build
go build -o ops main.go

# Install
go install

# Format
go fmt ./...

# Tidy modules
go mod tidy
```

### Running Tests

```bash
go test ./...
```

### Key Dependencies

- **[urfave/cli/v3](https://github.com/urfave/cli)** - CLI framework with commands and subcommands
- **[manifoldco/promptui](https://github.com/manifoldco/promptui)** - Interactive prompts
- **[supabase-community/supabase-go](https://github.com/supabase-community/supabase-go)** - Supabase client for Go
- **[cloud.google.com/go/storage](https://pkg.go.dev/cloud.google.com/go/storage)** - Google Cloud Storage (indirect dependency)

## Architecture

### Authentication Flow

```
┌─────────┐                ┌──────────────┐              ┌──────────┐
│   CLI   │                │   Browser    │              │ Supabase │
└────┬────┘                └──────┬───────┘              └────┬─────┘
     │                            │                           │
     │ 1. Start callback server   │                           │
     ├───────────────────────────>│                           │
     │                            │                           │
     │ 2. Open OAuth URL          │                           │
     ├───────────────────────────>│                           │
     │                            │                           │
     │                            │ 3. Authenticate           │
     │                            ├──────────────────────────>│
     │                            │                           │
     │                            │ 4. Redirect with tokens   │
     │                            │<──────────────────────────┤
     │                            │                           │
     │ 5. Handle callback         │                           │
     │<───────────────────────────┤                           │
     │                            │                           │
     │ 6. Store tokens locally    │                           │
     │                            │                           │
```

### Deployment Flow

```
┌─────────┐         ┌────────┐         ┌────────────┐         ┌──────────┐
│   CLI   │         │ Docker │         │ Controller │         │ Platform │
└────┬────┘         └───┬────┘         └─────┬──────┘         └────┬─────┘
     │                  │                    │                     │
     │ 1. Build image   │                    │                     │
     ├─────────────────>│                    │                     │
     │                  │                    │                     │
     │ 2. Save & gzip   │                    │                     │
     ├─────────────────>│                    │                     │
     │                  │                    │                     │
     │ 3. Upload .tgz   │                    │                     │
     ├────────────────────────────────────────>                    │
     │                  │                    │                     │
     │ 4. Return FQIN   │                    │                     │
     │<────────────────────────────────────────                    │
     │                  │                    │                     │
     │ 5. Create deployment                  │                     │
     ├────────────────────────────────────────>                    │
     │                  │                    │                     │
     │                  │                    │ 6. Deploy container │
     │                  │                    ├────────────────────>│
     │                  │                    │                     │
     │ 7. Return service URL                 │                     │
     │<────────────────────────────────────────                    │
     │                  │                    │                     │
```

### API Endpoints

The CLI interacts with these controller endpoints:

- `POST /api/v1/container-images` - Upload compressed container image
- `POST /api/v1/deployments` - Create new deployment

Both require Bearer token authentication in the `Authorization` header.

## CI/CD

The project uses Google Cloud Build for continuous deployment. On each tagged release:

1. **Build** - Compiles Go binary for linux/amd64 with embedded Supabase credentials
2. **Compress** - Creates `.tgz` archive of the binary
3. **Release** - Creates GitHub release and uploads binary as asset

### Triggering a Release

```bash
# Tag a new version
git tag v1.0.0
git push origin v1.0.0

# Cloud Build automatically:
# - Builds the binary
# - Creates GitHub release
# - Uploads ops_linux_amd64.tgz
```

### Cloud Build Configuration

See [`cloudbuild.yaml`](./cloudbuild.yaml) for the complete pipeline definition.

**Required Secrets:**
- `ops-github-access-token` - GitHub personal access token with repo scope
- `NUXT_PUBLIC_SUPABASE_URL` - Supabase project URL
- `NUXT_PUBLIC_SUPABASE_API_KEY` - Supabase anonymous key

## Contributing

Contributions are welcome! Please follow these guidelines:

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Commit** your changes (`git commit -m 'Add amazing feature'`)
4. **Push** to the branch (`git push origin feature/amazing-feature`)
5. **Open** a Pull Request

### Code Style

- Run `go fmt ./...` before committing
- Follow standard Go conventions and idioms
- Add tests for new functionality
- Update documentation as needed

### Development Workflow

```bash
# 1. Make changes to code
# 2. Format
just fmt

# 3. Test locally
just run deploy

# 4. Build
just build

# 5. Test binary
./ops deploy
```

## Troubleshooting

### Authentication Issues

**Problem:** `authentication failed: please log in again`

**Solution:**
```bash
# Remove existing tokens and re-authenticate
rm ~/.config/ops/credentials.json
ops login
```

### Docker Issues

**Problem:** `failed to build container image`

**Solution:**
- Ensure Docker daemon is running: `docker ps`
- Verify Dockerfile exists in current directory
- Check Docker permissions (may need `sudo` or user in `docker` group)

### Connection Issues

**Problem:** `failed to transmit compressed image: connection refused`

**Solution:**
- Check network connectivity
- Verify controller URL in config
- Ensure controller is accessible: `curl -I http://34.58.48.78`

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [urfave/cli](https://github.com/urfave/cli)
- Authentication powered by [Supabase](https://supabase.com)
- UI components from [promptui](https://github.com/manifoldco/promptui)

---

**Maintainer:** Joshua Castellucci ([@0p5dev](https://github.com/0p5dev))

**Repository:** [github.com/0p5dev/ops](https://github.com/0p5dev/ops)
