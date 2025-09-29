# Basic Usage Examples

Simple, practical examples of k0da usage.

## Basic Cluster Creation

### Default Cluster

```bash
# Create a cluster with default settings (auto-generated name)
k0da create
```

### Named Cluster

```bash
# Create a cluster with a specific name
k0da create my-project
# or using flag
k0da create --name my-project
```

### Multiple Clusters

```bash
# Create clusters for different purposes
k0da create dev
k0da create test
k0da create staging
```

## Configuration Features

### Basic Config File

```yaml
# basic-cluster.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: my-cluster
```

```bash
k0da create --config basic-cluster.yaml
```

### Multi-node Cluster

```yaml
# multi-node.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: multi-cluster
  nodes:
    - role: controller
    - role: worker
    - role: worker
```

```bash
k0da create multi --config multi-node.yaml
```

### Custom Networking

```yaml
# custom-network.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      spec:
        network:
          podCIDR: "192.168.0.0/16"
          serviceCIDR: "172.20.0.0/12"
```

### With Manifests

```yaml
# with-manifests.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: manifest-cluster
    manifests:
      - ./app/namespace.yaml
      - ./app/deployment.yaml
      - https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
```

```bash
k0da create manifest-test --config with-manifests.yaml
```

Manifests provide a simple way to install workloads directly into your cluster during creation. Both local files and remote URLs are supported, making it easy to bootstrap your cluster with essential components.

## Working with Clusters

### List and Switch

```bash
# See all clusters
k0da list

# See all clusters including stopped ones
k0da list --all

# Switch between clusters
k0da context dev
kubectl get nodes

k0da context test
kubectl get nodes
```

### Load Images

```bash
# Build and load your app
docker build -t my-app:latest .
k0da load image my-app:latest --name dev

# Load from tar archive
k0da load archive my-images.tar --name dev
```

### Cleanup

```bash
# Delete specific cluster
k0da delete dev

# Force delete without confirmation
k0da delete test --force
```

## Complete Workflow Example

```bash
# 1. Create cluster
k0da create my-app

# 2. Get kubeconfig (it's automatically available)
k0da kubeconfig --name my-app

# 3. Build and load image
docker build -t my-app:dev .
k0da load image my-app:dev --name my-app

# 4. Deploy
kubectl create deployment app --image=my-app:dev
kubectl expose deployment app --port=80 --type=NodePort

# 5. Test
kubectl get pods
kubectl get svc

# 6. When done
k0da delete my-app
```

That's it! Simple, straightforward usage without complexity.

## Installing Helm Charts

You can also install Helm charts directly through k0s configuration:

```yaml
# helm-cluster.yaml
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
            - name: prometheus
              url: https://prometheus-community.github.io/helm-charts
            charts:
            - name: ingress-nginx
              chartname: ingress-nginx/ingress-nginx
              version: "4.8.0"
              namespace: ingress-nginx
            - name: prometheus
              chartname: prometheus/kube-prometheus-stack
              version: "55.0.0"
              namespace: monitoring
```

```bash
k0da create helm-example --config helm-cluster.yaml
```

This automatically installs the specified Helm charts when the cluster starts up. For more details about Helm chart configuration options, see the [k0s Helm Charts documentation](https://docs.k0sproject.io/stable/helm-charts).