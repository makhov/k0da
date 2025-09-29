# Quick Start Guide

Get up and running with k0da in just a few minutes! This guide will walk you through creating your first k0s cluster.

## Before You Begin

Make sure you have completed the [installation](installation.md) and have the following ready:

- ✅ k0da installed and available in your PATH
- ✅ Docker or Podman running
- ✅ kubectl installed

## Your First Cluster

### Step 1: Create a Cluster

Create your first k0da cluster:

```bash
k0da create cluster
```

Or with a specific name:

```bash
k0da create cluster my-first-cluster
```

### Step 2: Verify Your Cluster

Check that your cluster was created successfully:

```bash
k0da list
```

You should see output like:
```
NAME              STATUS    NODES    VERSION
my-first-cluster  Running   1        v1.29.0+k0s.0
```

### Step 3: Get Kubeconfig

k0da configures kubectl to connect to your cluster automatically. To view the kubeconfig for the cluster run:

```bash
# Get kubeconfig and save to default location
k0da kubeconfig my-first-cluster
```

### Step 4: Interact with Your Cluster

Now you can use kubectl to interact with your cluster:

```bash
# Check cluster info
kubectl cluster-info

# Get nodes
kubectl get nodes

# Check system pods
kubectl get pods -A
```

Example output:
```bash
$ kubectl get nodes
NAME                  STATUS   ROLES           AGE   VERSION
my-first-cluster      Ready    control-plane   2m    v1.29.0+k0s.0

$ kubectl get pods -A
NAMESPACE            NAME                                         READY   STATUS    RESTARTS   AGE
kube-system          coredns-6f6b679f8f-abc123                   1/1     Running   0          2m
kube-system          konnectivity-agent-xyz789                   1/1     Running   0          2m
kube-system          kube-proxy-def456                          1/1     Running   0          2m
```

## Deploy Your First Application

Let's deploy a simple application to test everything is working:

```bash
# Create a deployment
kubectl create deployment hello-k0da --image=nginx:alpine

# Expose it as a service
kubectl expose deployment hello-k0da --port=80 --type=NodePort

# Check the deployment
kubectl get pods,svc
```

Get the service URL:

```bash
# Get the node port
kubectl get svc hello-k0da

# Access the service (replace NODE_PORT with actual port)
curl localhost:NODE_PORT
```

## Managing Your Cluster

### Switch Between Clusters

If you create multiple clusters, switch between them using:

```bash
# Create another cluster
k0da create my-second-cluster

# List all clusters
k0da list

# Switch context
k0da context my-second-cluster
```

### Load Container Images

Load local images into your cluster:

```bash
# Build a local image
docker build -t my-app:latest .

# Load into cluster
k0da load image my-app:latest --name my-first-cluster
```

### Clean Up

When you're done, clean up your clusters:

```bash
# Delete a specific cluster
k0da delete my-first-cluster

# Force delete without confirmation
k0da delete my-second-cluster --force
```

## What's Next?

Great! You've successfully created and managed your first k0da cluster. Here are some next steps:

### Learn More

- **[User Guide](user-guide/creating-clusters.md)** - Detailed cluster creation options
- **[CLI Reference](cli/k0da.md)** - Complete command reference
- **[Configuration](user-guide/configuration.md)** - Customize your clusters
- **[Examples](examples/basic.md)** - More real-world examples

### Advanced Topics

- **[Configuration](user-guide/configuration.md)** - Advanced configuration options
- **[CLI Reference](cli/k0da.md)** - Complete command reference

## Common Patterns

### Development Workflow

```bash
# Start your development session
k0da create cluster dev-cluster

# Load your application image
docker build -t my-app:dev .
k0da load image my-app:dev --cluster dev-cluster

# Deploy and test
kubectl apply -f manifests/

# When done, clean up
k0da delete dev-cluster
```

### Testing with Multiple Versions

```bash
# Test with different k0s versions
k0da create cluster test-v1-28 --k0s-version v1.28.0+k0s.0
k0da create cluster test-v1-29 --k0s-version v1.29.0+k0s.0

# Run your tests against each cluster
k0da context test-v1-28
kubectl apply -f test-manifests/
# ... run tests ...

k0da context test-v1-29  
kubectl apply -f test-manifests/
# ... run tests ...
```

!!! tip "Pro Tip"
    Use k0da in your development workflow to quickly spin up and tear down clusters for testing different scenarios, versions, or configurations!