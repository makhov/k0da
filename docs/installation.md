# Installation

This guide will help you install k0da on your system.

## Installation Methods

### Method 1: Download Pre-built Binaries (Recommended)

Download the latest release from GitHub:

```bash
# Download for Linux (AMD64)
curl -LO https://github.com/makhov/k0da/releases/latest/download/k0da-linux-amd64

# Download for macOS (ARM64)
curl -LO https://github.com/makhov/k0da/releases/latest/download/k0da-darwin-arm64

# Download for macOS (AMD64)
curl -LO https://github.com/makhov/k0da/releases/latest/download/k0da-darwin-amd64

# Download for Windows
curl -LO https://github.com/makhov/k0da/releases/latest/download/k0da-windows-amd64.exe
```

Make it executable and move to your PATH:

```bash
# Linux/macOS
chmod +x k0da-*
sudo mv k0da-* /usr/local/bin/k0da

# Verify installation
k0da version
```

### Method 2: Build from Source

If you prefer to build from source or want the latest development version:

```bash
# Clone the repository
git clone https://github.com/makhov/k0da.git
cd k0da

# Build the binary
make build

# Install to /usr/local/bin (optional)
sudo mv k0da-* /usr/local/bin/k0da
```

### Method 3: Using Go Install

Install directly using Go:

```bash
go install github.com/makhov/k0da@latest
```

!!! note
    Make sure your `$GOPATH/bin` is in your `$PATH` to use k0da directly.

## Verify Installation

After installation, verify that k0da is working correctly:

```bash
# Check version
k0da version

# Check available commands
k0da --help
```

You should see output similar to:

```
k0da version v1.0.0
commit: abc123def
built: 2024-01-15T10:30:00Z
go version: go1.23.0
```

## Container Runtime Setup

### Docker

Install Docker following the [official Docker installation guide](https://docs.docker.com/get-docker/).

Test your Docker installation:

```bash
docker run hello-world
```

### Podman

Install Podman following the [official Podman installation guide](https://podman.io/docs/installation).

For macOS users with Podman, ensure you're using a rootful machine:

```bash
podman machine set --rootful
podman machine stop
podman machine start
```

Test your Podman installation:

```bash
podman run hello-world
```

## kubectl Installation

Install kubectl to interact with your k0da clusters:

```bash
# macOS (using Homebrew)
brew install kubectl

# Linux
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/

# Windows (using Chocolatey)
choco install kubernetes-cli
```

Verify kubectl installation:

```bash
kubectl version --client
```

## Next Steps

Now that you have k0da installed, proceed to the [Quick Start Guide](quickstart.md) to create your first cluster!

## Troubleshooting

### Common Issues

#### "k0da: command not found"

Ensure the k0da binary is in your PATH. Check with:

```bash
echo $PATH
which k0da
```

#### Docker/Podman Permission Issues

Ensure your user is in the docker group (Linux) or Docker Desktop is running (macOS/Windows):

```bash
# Linux: Add user to docker group
sudo usermod -aG docker $USER
# Log out and log back in
```

#### Go Build Issues

Ensure you have Go 1.23+ installed:

```bash
go version
```

If you need to update Go, download it from [golang.org](https://golang.org/dl/).

For more help, check the [CLI Reference](cli/k0da.md) or create an issue on GitHub.