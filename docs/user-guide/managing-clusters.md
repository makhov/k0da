# Managing Clusters

This guide covers how to manage existing k0da clusters including listing, updating, and deleting clusters.

## Overview

Once you have created clusters with k0da, you'll need to manage them throughout their lifecycle. k0da provides simple commands to list all your clusters, update their configuration, and clean them up when no longer needed.

## Listing Clusters

Use the `list` command to see all your k0da clusters:

### Basic Listing

```bash
# List all running clusters
k0da list

# List all clusters including stopped ones
k0da list --all
```

### Detailed Information

```bash
# Show detailed cluster information
k0da list --verbose
```

The verbose output includes:

- Cluster status (running/stopped)
- k0s version
- Node count
- Container IDs
- Port mappings
- Creation time

### Example Output

```bash
$ k0da list
NAME           STATUS    NODES   VERSION         AGE
dev-cluster    Running   1       v1.33.4-k0s.0   2h
test-env       Running   3       v1.33.4-k0s.0   1h
staging        Stopped   2       v1.33.4-k0s.0   3d

$ k0da list --verbose
NAME           STATUS    NODES   VERSION         CONTAINERS                 PORTS                    AGE
dev-cluster    Running   1       v1.33.4-k0s.0   k0da-dev-cluster-control   0.0.0.0:6443->6443/tcp   2h
test-env       Running   3       v1.33.4-k0s.0   k0da-test-env-control,     0.0.0.0:6444->6443/tcp   1h
                                                 k0da-test-env-worker,
                                                 k0da-test-env-worker2
```

## Updating Clusters

The `update` command allows you to modify existing cluster configuration:

### Basic Updates

```bash
# Update cluster with new configuration file
k0da update my-cluster --config updated-config.yaml

# Update to a different k0s version
k0da update my-cluster --image quay.io/k0sproject/k0s:v1.33.5-k0s.0

# Update cluster by name
k0da update --name my-cluster --config new-config.yaml
```

### Update Process

When you update a cluster, k0da:

1. Validates the new configuration
2. Updates the effective k0s configuration using k0s dynamic config
3. Applies any new manifests without restart

### Update Examples

```bash
# Update with timeout for readiness check
k0da update production --config prod-v2.yaml --timeout 120s

# Update development cluster with new manifests
k0da update dev --config dev-with-ingress.yaml

# Quick version update
k0da update test --image quay.io/k0sproject/k0s:latest
```

### What Can Be Updated

- k0s configuration (ClusterConfig)
- Manifest files
- Helm charts (via k0s configuration)

### What Cannot Be Updated

- k0s version (requires restart)
- Node topology (adding/removing nodes)
- Network configuration (pod/service CIDRs)
- Container runtime settings
- Volume mounts

!!! note
    For changes that cannot be updated, you'll need to delete and recreate the cluster.

## Deleting Clusters

Remove clusters when they're no longer needed:

### Basic Deletion

```bash
# Delete a specific cluster
k0da delete my-cluster

# Delete using the name flag
k0da delete --name my-cluster
```

### Force Deletion

```bash
# Skip confirmation prompt
k0da delete my-cluster --force

# Delete multiple clusters
k0da delete dev-cluster test-env staging --force
```

### Deletion Process

When deleting a cluster, k0da:
1. Stops all cluster containers
2. Removes the containers
3. Cleans up networks (if not shared)
4. Removes cluster data

### Examples

```bash
# Interactive deletion (asks for confirmation)
k0da delete old-cluster

# Force deletion without confirmation
k0da delete temp-cluster --force

# Delete multiple clusters at once
k0da delete cluster1 cluster2 cluster3 --force
```

## Cluster Context Management

Switch between different cluster contexts:

### Context Operations

```bash
# Switch to a specific cluster context
k0da context my-cluster

# List available contexts
kubectl config get-contexts

# Get current context
kubectl config current-context
```

### Kubeconfig Management

```bash
# Get kubeconfig for a specific cluster
k0da kubeconfig my-cluster

# Merge cluster kubeconfig with your default kubeconfig
k0da kubeconfig my-cluster --merge

# Save kubeconfig to a specific file
k0da kubeconfig my-cluster > my-cluster-kubeconfig.yaml
```

## Best Practices

### Regular Maintenance

1. **List clusters regularly** to keep track of running clusters:
   ```bash
   k0da list --all
   ```

2. **Clean up unused clusters** to free resources:
   ```bash
   k0da delete old-cluster test-cluster --force
   ```

3. **Update clusters** to keep k0s versions current:
   ```bash
   k0da update production --image quay.io/k0sproject/k0s:v1.33.5-k0s.0
   ```

### Naming Conventions

Use consistent naming for easier management:

```bash
# Environment-based naming
k0da list | grep dev-
k0da list | grep prod-
k0da list | grep test-

# Feature-based naming
k0da delete feature-auth feature-payment --force

# Temporary clusters
k0da delete temp-* --force
```

### Automation

Integrate cluster management into scripts:

```bash
#!/bin/bash
# Cleanup script for temporary clusters

# List all temporary clusters
temp_clusters=$(k0da list | grep "temp-" | awk '{print $1}')

if [ ! -z "$temp_clusters" ]; then
    echo "Cleaning up temporary clusters: $temp_clusters"
    k0da delete $temp_clusters --force
else
    echo "No temporary clusters to clean up"
fi
```

### Monitoring

Keep track of cluster resource usage:

```bash
# Check cluster status
k0da list --verbose

# Check Docker/Podman resource usage
docker stats $(docker ps --filter "label=io.k0da.cluster" --format "{{.Names}}")

# Monitor cluster health
kubectl get nodes
kubectl get pods -A
kubectl top nodes  # if metrics-server is installed
```

## Troubleshooting

### Common Issues

**Cluster not listed:**
```bash
# Check if containers exist
docker ps -a --filter "label=io.k0da.cluster"

# Check container logs
docker logs k0da-cluster-name-control
```

**Update fails:**
```bash
# Check configuration validity
k0da update cluster-name --config config.yaml --dry-run

# Check cluster status
k0da list --verbose
kubectl get nodes
```

**Delete fails:**
```bash
# Force stop and remove containers
docker stop $(docker ps -q --filter "label=io.k0da.cluster.name=cluster-name")
docker rm $(docker ps -aq --filter "label=io.k0da.cluster.name=cluster-name")
```

### Recovery

If a cluster becomes unresponsive:

```bash
# Try updating to reset configuration
k0da update problematic-cluster --config working-config.yaml

# If update fails, delete and recreate
k0da delete problematic-cluster --force
k0da create problematic-cluster --config working-config.yaml
```

## Next Steps

- **[Configuration](configuration.md)** - Learn about advanced configuration options
- **[Loading Images](loading-images.md)** - Get your applications into clusters
- **[CLI Reference](../cli/k0da.md)** - Detailed command reference