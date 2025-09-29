# Creating Clusters

This guide covers everything you need to know about creating k0s clusters with k0da.

## Overview

k0da makes it easy to create local Kubernetes clusters using k0s. Whether you need a simple single-node cluster for development or a complex multi-node setup for testing, k0da has you covered.

## Basic Cluster Creation

### Your First Cluster

The simplest way to create a cluster:

```bash
k0da create cluster my-cluster
```

This creates:
- A single-node cluster named "my-cluster"
- Uses the latest k0s version
- Default networking configuration
- Ready to use with kubectl

### Specifying k0s Version

```bash
# Use a specific version
k0da create cluster stable --k0s-version v1.29.0+k0s.0

# Use the latest version explicitly
k0da create cluster latest --k0s-version latest

# Use a specific patch version
k0da create cluster patch --k0s-version v1.28.5+k0s.0
```

## Multi-Node Clusters

### Simple Multi-Node Setup

```bash
# Create a 3-node cluster (1 control plane + 2 workers)
k0da create cluster multi --nodes 3

# Create a larger cluster
k0da create cluster big --nodes 5
```

### Custom Node Configuration

```bash
# Specify control plane and worker nodes separately
k0da create cluster custom \
  --control-plane-nodes 3 \
  --worker-nodes 3
```

## Networking Configuration

### Default Network Settings

By default, k0da uses:
- **Pod CIDR**: 10.244.0.0/16
- **Service CIDR**: 10.96.0.0/12
- **CNI**: Calico

### Custom Networking

```bash
# Custom network ranges
k0da create cluster network-test \
  --pod-subnet 192.168.0.0/16 \
  --service-subnet 172.20.0.0/16

# Disable default CNI (bring your own)
k0da create cluster custom-cni \
  --disable-default-cni
```

### Port Mappings

Expose cluster services on host ports:

```bash
# Map common web ports
k0da create cluster web-cluster \
  --port-mapping 8080:80 \
  --port-mapping 8443:443

# Map multiple ports
k0da create cluster service-cluster \
  --port-mapping 3000:3000 \
  --port-mapping 5432:5432 \
  --port-mapping 6379:6379
```

## Storage and Volumes

### Host Path Mounts

Mount directories from your host system:

```bash
# Mount current directory as workspace
k0da create cluster dev \
  --volume $(pwd):/workspace

# Mount multiple directories
k0da create cluster full-dev \
  --volume $(pwd):/app \
  --volume /Users/me/data:/data \
  --volume /Users/me/config:/config:ro
```

### Volume Options

```bash
# Read-only mount
k0da create cluster readonly \
  --volume /host/config:/etc/config:ro

# Read-write mount (default)
k0da create cluster readwrite \
  --volume /host/data:/data:rw

# Mount with specific options
k0da create cluster advanced \
  --volume /host/data:/data:rw,Z
```

### Temporary Filesystems

```bash
# Create tmpfs mounts for performance
k0da create cluster fast \
  --tmpfs /tmp \
  --tmpfs /var/cache:size=1g,noexec
```

## Advanced Configuration

### Custom k0s Configuration

Create configuration files for different scenarios:

#### Development Configuration

```yaml
# dev-k0s.yaml
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: dev-cluster
spec:
  api:
    port: 6443
  network:
    podCIDR: "10.244.0.0/16"
    serviceCIDR: "10.96.0.0/12"
    kubeProxy:
      mode: "iptables"
  storage:
    type: "etcd"
```

#### Production-like Configuration

```yaml
# prod-like-k0s.yaml  
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: production-test
spec:
  api:
    port: 6443
    sans:
    - "api.test.local"
  controllerManager:
    extraArgs:
      cluster-signing-duration: "8760h"
  scheduler:
    extraArgs:
      bind-address: "0.0.0.0"
  network:
    podCIDR: "192.168.0.0/16"
    serviceCIDR: "172.20.0.0/16"
    kubeProxy:
      mode: "ipvs"
  storage:
    type: "etcd"
    etcd:
      peerAddress: "0.0.0.0"
```

#### Configuration with Extensions

```yaml
# extensions-k0s.yaml
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: full-featured
spec:
  api:
    port: 6443
  network:
    podCIDR: "10.244.0.0/16" 
    serviceCIDR: "10.96.0.0/12"
  extensions:
    helm:
      repositories:
      - name: bitnami
        url: https://charts.bitnami.com/bitnami
      charts:
      - name: metrics-server
        chartname: bitnami/metrics-server
        version: "6.5.0"
        namespace: kube-system
```

```bash
# Use configurations
k0da create cluster dev --config dev-k0s.yaml
k0da create cluster prod-test --config prod-like-k0s.yaml
k0da create cluster full --config extensions-k0s.yaml
```

### Environment Variables

Set environment variables in cluster containers:

```bash
k0da create cluster env-cluster \
  --env DEBUG=true \
  --env LOG_LEVEL=info \
  --env CUSTOM_VAR=value
```

### Container Runtime Selection

```bash
# Force Docker usage
k0da create cluster docker-cluster --container-runtime docker

# Force Podman usage  
k0da create cluster podman-cluster --container-runtime podman
```

## Cluster Naming and Organization

### Naming Conventions

Choose meaningful names for your clusters:

```bash
# Environment-based naming
k0da create cluster dev-frontend
k0da create cluster staging-api  
k0da create cluster test-integration

# Feature-based naming
k0da create cluster feature-auth
k0da create cluster fix-memory-leak
k0da create cluster experiment-gpu

# Version-based naming
k0da create cluster k8s-1-29
k0da create cluster k8s-1-28-test
```

### Project Organization

Organize clusters by project:

```bash
# Create project-specific clusters
k0da create cluster myapp-dev --nodes 1
k0da create cluster myapp-test --nodes 3
k0da create cluster myapp-perf --nodes 5

# Use consistent naming
k0da create cluster ${PROJECT}-${ENV}-${USER}
```

## Performance Considerations

### Resource Optimization

```bash
# Minimal cluster for resource-constrained environments
k0da create cluster minimal \
  --nodes 1 \
  --k0s-version v1.29.0+k0s.0

# Performance cluster with more resources
k0da create cluster performance \
  --nodes 3 \
  --tmpfs /tmp \
  --tmpfs /var/log
```

### Startup Time

```bash
# Wait for cluster to be ready
k0da create cluster fast --wait --timeout 300

# Don't wait (faster creation, manual verification needed)  
k0da create cluster async --no-wait
```

## Development Workflows

### Iterative Development

```bash
# Create development cluster
k0da create cluster dev \
  --nodes 1 \
  --volume $(pwd):/workspace \
  --port-mapping 8080:80

# Set up kubeconfig
k0da kubeconfig dev --merge

# Your development cycle:
# 1. Make code changes
# 2. Build image: docker build -t myapp:dev .
# 3. Load image: k0da load image myapp:dev --cluster dev
# 4. Deploy: kubectl apply -f manifests/
# 5. Test: curl localhost:8080
```

### Testing Workflows

```bash
# Create test environment
k0da create cluster test-env \
  --nodes 3 \
  --k0s-version v1.29.0+k0s.0 \
  --wait

# Run your test suite
k0da kubeconfig test-env --merge
kubectl config use-context k0da-test-env

# Deploy test applications
kubectl apply -f test-manifests/

# Run tests
./run-integration-tests.sh

# Cleanup
k0da delete test-env
```

## Troubleshooting Cluster Creation

### Common Issues

#### Image Pull Problems

```bash
# Check if image exists
docker pull k0sproject/k0s:v1.29.0-k0s.0

# Use different image
k0da create cluster test --image k0sproject/k0s:latest
```

#### Port Conflicts

```bash
# k0da automatically finds available ports, but you can check:
netstat -ln | grep 6443

# Force specific port
k0da create cluster test --api-port 6444
```

#### Resource Issues

```bash
# Check available resources
docker system df
docker system prune  # Clean up if needed

# Create smaller cluster
k0da create cluster small --nodes 1
```

### Debug Mode

```bash
# Enable verbose logging
k0da create cluster debug-cluster --verbose

# Check cluster status
k0da list
kubectl get nodes -o wide
kubectl get pods -A
```

### Validation

After creating a cluster, validate it's working:

```bash
# Basic cluster validation
kubectl cluster-info
kubectl get nodes
kubectl get pods -A

# Deploy test application
kubectl create deployment nginx --image=nginx
kubectl expose deployment nginx --port=80 --type=NodePort
kubectl get svc nginx
```

## Best Practices

### Development

1. **Use descriptive names** that include purpose and environment
2. **Mount your code directory** for easier development
3. **Use single nodes** for development to save resources
4. **Clean up regularly** to avoid resource exhaustion

### Testing

1. **Use multi-node clusters** to test real-world scenarios
2. **Pin k0s versions** for reproducible tests
3. **Automate cluster creation** in CI/CD pipelines
4. **Include cluster cleanup** in test scripts

### Production-like Testing

1. **Match node counts** to production topology
2. **Use same k0s version** as production
3. **Test with resource constraints** similar to production
4. **Include networking complexity** (ingress, service mesh, etc.)

## Configuration Examples

## Manifests and Workload Installation

### Using Manifests

Manifests provide a simple way to install workloads directly into your cluster during creation. This is particularly useful for bootstrapping clusters with essential components like ingress controllers, monitoring tools, or your application deployments.

#### Local Manifests

```yaml
# with-local-manifests.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: app-cluster
    manifests:
      - ./manifests/namespace.yaml
      - ./manifests/configmap.yaml
      - ./manifests/deployment.yaml
      - ./manifests/service.yaml
```

#### Remote Manifests

```yaml
# with-remote-manifests.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: ingress-cluster
    manifests:
      - https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
      - https://github.com/jetstack/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

#### Mixed Manifests

```yaml
# mixed-manifests.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: full-cluster
    manifests:
      - ./local/namespace.yaml              # Local file
      - ./local/app-config.yaml            # Local file
      - https://example.com/remote-app.yaml # Remote file
```

```bash
# Create clusters with manifests
k0da create app --config with-local-manifests.yaml
k0da create ingress --config with-remote-manifests.yaml
k0da create full --config mixed-manifests.yaml
```

**Key benefits:**
- Workloads are installed automatically during cluster creation
- Both local files and remote URLs are supported
- Manifests are applied in the order they appear in the list
- Perfect for creating reproducible cluster setups

## Installing Helm Charts

For more complex applications, you can install Helm charts directly through k0s configuration. This eliminates the need to manually add Helm repositories and install charts after cluster creation.

### Basic Helm Configuration

```yaml
# helm-basic.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: helm-cluster
      spec:
        extensions:
          helm:
            repositories:
            - name: ingress-nginx
              url: https://kubernetes.github.io/ingress-nginx
            charts:
            - name: ingress-nginx
              chartname: ingress-nginx/ingress-nginx
              version: "4.8.0"
              namespace: ingress-nginx
```

### Advanced Helm Configuration

```yaml
# helm-advanced.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: monitoring-cluster
      spec:
        extensions:
          helm:
            repositories:
            - name: prometheus
              url: https://prometheus-community.github.io/helm-charts
            - name: grafana
              url: https://grafana.github.io/helm-charts
            charts:
            - name: prometheus
              chartname: prometheus/kube-prometheus-stack
              version: "55.0.0"
              namespace: monitoring
              values: |
                grafana:
                  adminPassword: admin123
                  service:
                    type: NodePort
            - name: loki
              chartname: grafana/loki-stack
              version: "2.9.0"
              namespace: logging
```

```bash
# Create clusters with Helm charts
k0da create helm-basic --config helm-basic.yaml
k0da create monitoring --config helm-advanced.yaml
```

**Helm chart features:**
- Automatically adds repositories and installs charts
- Supports custom values for chart configuration
- Charts are installed during cluster initialization
- Perfect for complex application stacks

For more details about Helm chart configuration options, including advanced features like custom values files and chart dependencies, see the [k0s Helm Charts documentation](https://docs.k0sproject.io/stable/helm-charts).

### Machine Learning Workloads

```bash
# Cluster for ML with GPU support (if available)
k0da create cluster ml-cluster \
  --nodes 2 \
  --volume /data:/data \
  --volume /models:/models \
  --env CUDA_VISIBLE_DEVICES=all
```

### Database Testing

```bash
# Cluster for stateful applications
k0da create cluster db-test \
  --nodes 3 \
  --volume /host/data:/var/lib/postgresql \
  --port-mapping 5432:5432 \
  --config stateful-k0s.yaml
```

## Next Steps

- **[Managing Clusters](managing-clusters.md)** - Learn to manage existing clusters
- **[Loading Images](loading-images.md)** - Get your applications into clusters
- **[Configuration](configuration.md)** - Advanced k0da configuration
- **[CLI Reference](../cli/k0da_create.md)** - Detailed create command reference