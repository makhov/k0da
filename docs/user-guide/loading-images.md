# Loading Images

This guide covers how to load container images into your k0da clusters for use by your applications.

## Overview

k0da clusters run in containers with their own containerd instance. To use custom images in your cluster, you need to load them from your local Docker/Podman daemon or from image archives.

## Loading Images from Docker/Podman

Use `k0da load image` to pull and load images directly:

### Basic Image Loading

```bash
# Load an image from a registry
k0da load image nginx:latest

# Load into a specific cluster
k0da load image redis:7-alpine --name dev-cluster

# Load your custom application image
k0da load image myapp:v1.0.0
```

### Multiple Images

```bash
# Load multiple images
k0da load image nginx:latest
k0da load image postgres:15
k0da load image myapp:latest
```

## Loading from Archives

Use `k0da load archive` to load images from tar archives or OCI layout directories:

### Tar Archives

```bash
# Create an image archive first
docker save myapp:latest -o myapp.tar

# Load the archive into cluster
k0da load archive myapp.tar

# Load into specific cluster
k0da load archive myapp.tar --name production
```

### OCI Layout Directories

```bash
# Load from OCI layout directory
k0da load archive ./oci-layout-dir
```

## Common Workflows

### Development Workflow

```bash
# 1. Build your application image
docker build -t myapp:dev .

# 2. Load into development cluster
k0da load image myapp:dev --name dev-cluster

# 3. Deploy your application
kubectl apply -f deployment.yaml
```

### Multi-Cluster Deployment

```bash
# Load same image into multiple clusters
k0da load image myapp:v1.2.3 --name staging
k0da load image myapp:v1.2.3 --name production
k0da load image myapp:v1.2.3 --name test-env
```

### Air-Gapped Environments

```bash
# 1. Save images to archives on connected machine
docker save nginx:latest postgres:15 myapp:latest -o images.tar

# 2. Transfer archive to air-gapped environment
# 3. Load from archive
k0da load archive images.tar
```

## Best Practices

### Image Tagging

- Use specific tags instead of `latest` for reproducibility
- Tag images with version numbers or commit SHAs
- Use consistent naming conventions

```bash
# Good: specific versions
k0da load image myapp:v1.2.3
k0da load image nginx:1.24-alpine

# Avoid: vague tags
k0da load image myapp:latest
k0da load image nginx
```

### Efficient Loading

- Load images before deploying applications
- Use multi-stage Docker builds to reduce image size
- Load shared base images once and reuse

### Automation

Integrate image loading into your deployment scripts:

```bash
#!/bin/bash
CLUSTER_NAME="dev-cluster"
APP_VERSION="v1.0.0"

# Build application
docker build -t myapp:$APP_VERSION .

# Load into cluster
k0da load image myapp:$APP_VERSION --name $CLUSTER_NAME

# Deploy application
kubectl apply -f k8s/deployment.yaml
```