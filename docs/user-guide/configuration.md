# Configuration

This guide covers all configuration options available in k0da for creating and managing clusters.

## Configuration File Structure

k0da uses its own configuration format that wraps k0s configuration and adds cluster-specific options:

```yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    version: string              # Optional: k0s version
    image: string               # Optional: k0s image override  
    args: []string              # Optional: extra k0s arguments
    config: {}                  # k0s configuration (ClusterConfig)
    manifests: []string         # Optional: list of manifest files/URLs
  nodes: []NodeConfig          # Optional: multi-node configuration
  options:
    network: string             # Optional: container network name
```

## k0s Section

The `k0s` section contains all k0s-related configuration:

### Version and Image

```yaml
spec:
  k0s:
    # Specify k0s version (will use corresponding image)
    version: v1.33.4-k0s.0
    
    # OR specify image directly (overrides version)
    image: quay.io/k0sproject/k0s:v1.33.4-k0s.0
    
    # Extra arguments passed to k0s
    args: ["--debug", "--verbose"]
```

### k0s Configuration

The `config` section contains standard k0s ClusterConfig. Refer to the [k0s documentation](https://docs.k0sproject.io/) for all available options.

```yaml
spec:
  k0s:
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: my-cluster
      spec:
        # All standard k0s configuration options
        api:
          port: 6443
          sans: []
        network:
          podCIDR: "10.244.0.0/16"
          serviceCIDR: "10.96.0.0/12"
          provider: "calico"
          kubeProxy:
            mode: "iptables"
        storage:
          type: "etcd"
        telemetry:
          enabled: false
        extensions:
          helm:
            repositories: []
            charts: []
```

### Manifests

Manifests are YAML files applied automatically during cluster startup:

```yaml
spec:
  k0s:
    manifests:
      # Local files (relative to config file location)
      - ./manifests/namespace.yaml
      - ./manifests/deployment.yaml
      
      # Absolute paths
      - /path/to/manifest.yaml
      
      # Remote URLs
      - https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
      - https://github.com/jetstack/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

!!! warning 

    All manifests should specify namespace if applicable. Manifests without namespace won't be applied.


**Manifest features:**
 
- Applied in the order specified
- Support both local files and remote URLs
- Mounted read-only at `/var/lib/k0s/manifests/k0da` in the container

## Nodes Section

Define multi-node cluster topology:

### Basic Multi-Node

```yaml
spec:
  nodes:
    - role: controller
    - role: worker
    - role: worker
```

### Advanced Node Configuration

```yaml
spec:
  nodes:
    - role: controller
      image: quay.io/k0sproject/k0s:v1.33.4-k0s.0  # Override image for this node
      args: ["--labels=k0sproject.io/foo=bar"]      # Extra k0s arguments
      ports:
        - containerPort: 6443
          hostPort: 16443
          hostIP: "127.0.0.1"    # Optional
      mounts:
        - type: bind
          source: /host/path
          target: /container/path
          options: ["ro"]
      env:
        DEBUG: "true"
        LOG_LEVEL: "info"
      labels:
        purpose: "demo"
        environment: "dev"
    
    - role: worker
      args: ["--kubelet-extra-args=--max-pods=50"]
      env:
        NODE_TYPE: "worker"
```

**Node configuration options:**

- `role`: `controller` or `worker`
- `image`: Override k0s image for specific node
- `args`: Extra arguments for k0s command
- `ports`: Port mappings from container to host
- `mounts`: Volume mounts into the container
- `env`: Environment variables
- `labels`: Container labels

### Port Mappings

```yaml
ports:
  - containerPort: 6443        # Required: port inside container
    hostPort: 16443           # Optional: host port (auto-assigned if not specified)
    hostIP: "127.0.0.1"       # Optional: host IP to bind to
  - containerPort: 80
    hostPort: 8080
  - containerPort: 443
    # hostPort will be auto-assigned
```

### Volume Mounts

```yaml
mounts:
  - type: bind                 # Mount type (usually 'bind')
    source: /host/data         # Path on host
    target: /data              # Path in container
    options: ["rw"]            # Mount options: ro, rw, etc.
  
  - type: bind
    source: ./local/config     # Relative to config file
    target: /etc/config
    options: ["ro"]
```

## Options Section

Global cluster options:

```yaml
spec:
  options:
    network: "my-network"      # Custom container network (default: k0da)
```

## Complete Configuration Examples

### Simple Development Cluster

```yaml
# dev-cluster.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    version: v1.33.4-k0s.0
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: dev-cluster
      spec:
        telemetry:
          enabled: false
```

### Production-Like Cluster

```yaml
# prod-cluster.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    version: v1.33.4-k0s.0
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: prod-cluster
      spec:
        api:
          port: 6443
          sans:
            - "api.prod.local"
            - "10.0.1.100"
        network:
          podCIDR: "192.168.0.0/16"
          serviceCIDR: "172.20.0.0/16"
          kubeProxy:
            mode: "ipvs"
        storage:
          type: "etcd"
          etcd:
            peerAddress: "0.0.0.0"
        telemetry:
          enabled: false
  nodes:
    - role: controller
      ports:
        - containerPort: 6443
          hostPort: 6443
    - role: worker
    - role: worker
```

### Full-Featured Cluster

```yaml
# full-cluster.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    version: v1.33.4-k0s.0
    args: ["--debug"]
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: full-cluster
      spec:
        api:
          port: 6443
          sans: ["api.local"]
        network:
          podCIDR: "10.244.0.0/16"
          serviceCIDR: "10.96.0.0/12"
          provider: "calico"
        telemetry:
          enabled: false
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
              - name: monitoring
                chartname: prometheus/kube-prometheus-stack
                version: "55.0.0"
                namespace: monitoring
    manifests:
      - ./manifests/namespace.yaml
      - ./manifests/configmap.yaml
      - https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
  nodes:
    - role: controller
      ports:
        - containerPort: 6443
          hostPort: 6443
        - containerPort: 80
          hostPort: 80
        - containerPort: 443
          hostPort: 443
      mounts:
        - type: bind
          source: ./data
          target: /data
          options: ["rw"]
      env:
        DEBUG: "true"
      labels:
        role: "control-plane"
    - role: worker
      env:
        NODE_TYPE: "worker"
      labels:
        role: "worker"
    - role: worker
      env:
        NODE_TYPE: "worker"
      labels:
        role: "worker"
  options:
    network: "custom-network"
```

## Network Configuration

### Pod and Service Networks

```yaml
spec:
  k0s:
    config:
      spec:
        network:
          podCIDR: "10.244.0.0/16"      # Pod network CIDR
          serviceCIDR: "10.96.0.0/12"   # Service network CIDR
          provider: "calico"             # CNI provider
          kubeProxy:
            mode: "iptables"             # or "ipvs"
```

### Custom CNI

```yaml
spec:
  k0s:
    config:
      spec:
        network:
          provider: "custom"    # Disable default CNI
          # Configure your own CNI via manifests
    manifests:
      - ./cni/custom-cni.yaml
```

## Storage Configuration

### Default Storage

```yaml
spec:
  k0s:
    config:
      spec:
        storage:
          type: "etcd"
```

### External etcd

```yaml
spec:
  k0s:
    config:
      spec:
        storage:
          type: "etcd"
          etcd:
            peerAddress: "0.0.0.0"
            externalCluster:
              endpoints:
                - "https://etcd1.example.com:2379"
                - "https://etcd2.example.com:2379"
              etcdPrefix: "/k0da"
              caFile: "/etc/etcd/ca.pem"
              certFile: "/etc/etcd/cert.pem"
              keyFile: "/etc/etcd/key.pem"
```

## API Server Configuration

### Basic API Configuration

```yaml
spec:
  k0s:
    config:
      spec:
        api:
          port: 6443                    # API server port
          k0sApiPort: 9443             # k0s API port
          sans:                        # Subject Alternative Names
            - "api.example.com"
            - "10.0.1.100"
            - "192.168.1.100"
```

### API Server Extra Args

```yaml
spec:
  k0s:
    config:
      spec:
        api:
          extraArgs:
            audit-log-path: "/var/log/audit.log"
            audit-log-maxage: "30"
            enable-admission-plugins: "NodeRestriction,ResourceQuota"
```

## Controller Components

### Controller Manager

```yaml
spec:
  k0s:
    config:
      spec:
        controllerManager:
          extraArgs:
            bind-address: "0.0.0.0"
            cluster-signing-duration: "8760h"
```

### Scheduler

```yaml
spec:
  k0s:
    config:
      spec:
        scheduler:
          extraArgs:
            bind-address: "0.0.0.0"
            v: "2"
```

## Worker Configuration

### Kubelet Configuration

```yaml
spec:
  nodes:
    - role: worker
      args: 
        - "--kubelet-extra-args=--max-pods=110"
        - "--kubelet-extra-args=--cluster-dns=10.96.0.10"
```

## Extensions

### Helm Charts

```yaml
spec:
  k0s:
    config:
      spec:
        extensions:
          helm:
            repositories:
              - name: bitnami
                url: https://charts.bitnami.com/bitnami
            charts:
              - name: nginx
                chartname: bitnami/nginx
                version: "15.0.0"
                namespace: web
                values: |
                  service:
                    type: NodePort
                  ingress:
                    enabled: true
```

### Storage Classes

```yaml
spec:
  k0s:
    config:
      spec:
        extensions:
          storage:
            create_default_storage_class: true
            default_storage_class: "local-path"
```

## Environment-Specific Configurations

### Development

```yaml
# config/dev.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      spec:
        telemetry:
          enabled: false
        api:
          port: 6443
```

### Staging

```yaml
# config/staging.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      spec:
        network:
          kubeProxy:
            mode: "ipvs"
        telemetry:
          enabled: false
  nodes:
    - role: controller
    - role: worker
    - role: worker
```

### Production

```yaml
# config/production.yaml
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    config:
      spec:
        api:
          sans:
            - "api.prod.company.com"
        network:
          podCIDR: "192.168.0.0/16"
          serviceCIDR: "172.20.0.0/16"
          kubeProxy:
            mode: "ipvs"
        storage:
          type: "etcd"
          etcd:
            peerAddress: "0.0.0.0"
        telemetry:
          enabled: false
        extensions:
          helm:
            repositories:
              - name: prometheus
                url: https://prometheus-community.github.io/helm-charts
            charts:
              - name: monitoring
                chartname: prometheus/kube-prometheus-stack
                namespace: monitoring
  nodes:
    - role: controller
      ports:
        - containerPort: 6443
          hostPort: 6443
    - role: worker
    - role: worker
    - role: worker
```

## Configuration Best Practices

### Security

1. **Disable telemetry** for privacy:
   ```yaml
   spec:
     k0s:
       config:
         spec:
           telemetry:
             enabled: false
   ```

2. **Use specific versions** for reproducibility:
   ```yaml
   spec:
     k0s:
       version: v1.33.4-k0s.0  # Pin specific version
   ```

3. **Configure API server SANs** for external access:
   ```yaml
   spec:
     k0s:
       config:
         spec:
           api:
             sans:
               - "your-domain.com"
               - "your-ip-address"
   ```

### Performance

1. **Use IPVS** for better performance:
   ```yaml
   spec:
     k0s:
       config:
         spec:
           network:
             kubeProxy:
               mode: "ipvs"
   ```

2. **Configure appropriate pod limits**:
   ```yaml
   spec:
     nodes:
       - role: worker
         args: ["--kubelet-extra-args=--max-pods=110"]
   ```

### Maintainability

1. **Use separate files** for different environments
2. **Keep manifests organized** in directories
3. **Use meaningful cluster names**
4. **Document custom configurations**

## Validation

k0da validates the configuration before creating clusters. Common validation errors:

- Invalid k0s version or image reference
- Malformed k0s ClusterConfig
- Invalid manifest file paths or URLs
- Port conflicts in node configuration
- Invalid mount paths or options

## References

- **k0s Configuration**: [k0s Documentation](https://docs.k0sproject.io/)
- **Helm Charts**: [k0s Helm Charts](https://docs.k0sproject.io/stable/helm-charts/)
- **Network Providers**: [k0s Network Providers](https://docs.k0sproject.io/stable/networking/)
- **Storage**: [k0s Storage](https://docs.k0sproject.io/stable/storage/)