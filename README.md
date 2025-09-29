# k0da

🚀 k0da (/ˈkoʊdə/) is a small CLI for creating and managing local Kubernetes clusters using k0s.

## Features

- Lightweight and fast
- Full k0s support with all its features
- Works with Docker or Podman

## Install

Download a pre-built binary from the latest [release](https://github.com/makhov/k0da/releases/latest).

Or install the latest version with `go install`:

```bash
go install github.com/makhov/k0da@latest
```

## Install from source

```bash
git clone https://github.com/makhov/k0da.git
cd k0da
go build -o k0da
./k0da version
```

## Quickstart

```bash
# Create a cluster with defaults
k0da create

# Create a named cluster
k0da create my-cluster
# or
k0da create --name my-cluster

# List clusters
k0da list

# Delete a cluster
k0da delete my-cluster
```
k0da is a CLI utility similar to kind but opinionated and based on k0s.
It provides an easy way to create and manage lightweight Kubernetes clusters
using k0s as the distribution.

Usage:
  k0da [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  context     Switch to a different k0da cluster context
  create      Create a new k0s cluster
  delete      Delete a k0s cluster
  help        Help about any command
  list        List all k0da clusters
  load        Load images into the k0s cluster
  update      Update an existing k0s cluster
  version     Print version information

Flags:
      --config string   config file (default is $HOME/.k0da.yaml)
  -h, --help            help for k0da
  -v, --version         version for k0da

Use "k0da [command] --help" for more information about a command.

```

## Create command

```bash
k0da create [flags]

Flags:
  -n, --name string      cluster name (default: k0da-cluster)
  -i, --image string     k0s image to use (overrides config)
  -c, --config string    path to k0da cluster config file (YAML)
  -w, --wait             wait for readiness (default true)
  -t, --timeout string   readiness timeout (default "60s")
```

## Cluster config (k0da)

- `spec.k0s.config` is a plain k0s configuration (exactly as k0s expects). k0da merges your values onto a sensible default k0s config and writes the result to `/etc/k0s/k0s.yaml`, starting k0s with `--config /etc/k0s/k0s.yaml`.
- `spec.k0s.manifests` is a list of YAML files (absolute or relative to the cluster config file) that are bind-mounted read-only into `/var/lib/k0s/manifests/k0da`. Files are mounted with a numeric prefix to preserve the list order (e.g. `000_file.yaml`, `001_other.yaml`).
- You can also set extra k0s args in `spec.k0s.args`, and node-level `args`, `ports`, `mounts`, `env`, and `labels`.

Example (`cluster.yaml`):

```yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    # You can specify either image or version. If neither set, a default is used.
    version: v1.33.3-k0s.0
    args: ["--debug"]
    # This is a plain k0s config. It is merged with defaults and written to /etc/k0s/k0s.yaml
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: my-k0s-cluster
      spec:
        network:
          provider: calico
    # Optional: additional Kubernetes manifests to apply via k0s manifests
    manifests:
      - ./manifests/00-namespace.yaml
      - ./manifests/10-app.yaml
      - https://raw.githubusercontent.com/haproxytech/kubernetes-ingress/refs/heads/master/deploy/haproxy-ingress.yaml

  nodes:
    - role: controller
      # Optional: override the image for this node
      image: quay.io/k0sproject/k0s:v1.33.2-k0s.0
      # Extra flags appended to the k0s controller command
      args: ["--labels=\"k0sproject.io/foo=bar,k0sproject.io/other=xyz\""]
      # Port mappings (hostIP/hostPort optional). 6443/tcp is added by default.
      ports:
        - containerPort: 6443
          hostPort: 16443
      # Additional mounts
      mounts:
        - type: bind
          source: /path/on/host
          target: /mnt/in/container
          options: ["ro"]
      # Environment variables inside the node container
      env:
        FOO: "bar"
      # Extra container labels
      labels:
        purpose: demo

  options:
    network: kind # the network to use for the node containers (default: k0da)
```

Create the cluster from the file:

```bash
k0da create -c ./cluster.yaml
```

Notes:
- Manifest paths may be absolute or relative to the config file location.
- Manifests are mounted read-only into `/var/lib/k0s/manifests/k0da` and k0s processes them automatically.
- Order is preserved by prefixing filenames with a zero-padded index.

## Image loading

```bash
# Load an image archive (tar or OCI layout dir) into the cluster
k0da load archive ./my-images.tar -n demo

# Load a local image from your host runtime (Docker/Podman) into the cluster
# This saves the local image to a temporary tar and imports it into k0s containerd
k0da load image docker.io/library/nginx:alpine -n demo
```

## Update command

Use `k0da update` to apply changes to manifests and k0s configuration without recreating the container.

Example:

```yaml
# cluster.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      spec:
        telemetry:
          enabled: false
    manifests:
      - ./manifests/00-namespace.yaml
      - ./manifests/10-app.yaml
```

```bash
# Apply updates (manifests and dynamic config)
k0da update -n my-cluster -c ./cluster.yaml
```

## Runtime selection

By default k0da auto-detects Docker or Podman. You can override via env vars:

```bash
export K0DA_RUNTIME=docker        # or podman
export K0DA_SOCKET=unix:///var/run/docker.sock   # or podman socket/URI
```

## License

MIT
