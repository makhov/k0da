# k0da Documentation

🚀 **k0da** (/ˈkoʊdə/) is a small CLI for creating and managing local Kubernetes clusters using k0s.

## Key Features

- **Lightweight and fast** - Built on k0s for minimal resource usage
- **Full k0s support** - Access to all k0s features and capabilities
- **Container runtime flexibility** - Works with Docker or Podman
- **Simple CLI interface** - Easy-to-use commands for cluster management
- **Local development focused** - Perfect for development and testing workflows

## Quick Start

```bash
# Create a new cluster
k0da create cluster my-cluster

# List clusters
k0da list

# Get kubeconfig
k0da kubeconfig my-cluster

# Delete cluster
k0da delete my-cluster
```

## Why k0da?

k0da is designed for developers who need quick, reliable local Kubernetes environments powered by k0s. It focuses on simplicity and efficiency, making it easy to spin up clusters for development, testing, and experimentation.

## Getting Started

Ready to get started? Head over to our [Installation Guide](installation.md) to set up k0da on your system.

## Community and Support

- **GitHub**: [makhov/k0da](https://github.com/makhov/k0da)
- **Issues**: Report bugs or request features on [GitHub Issues](https://github.com/makhov/k0da/issues)
- **Discussions**: Join the conversation in [GitHub Discussions](https://github.com/makhov/k0da/discussions)

## License

k0da is open source software licensed under the [Apache License 2.0](https://github.com/makhov/k0da/blob/main/LICENSE).