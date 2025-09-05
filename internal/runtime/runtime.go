package runtime

import (
	"context"
	"strings"
)

// PortSpec describes a port to publish from container to host.
type PortSpec struct {
	ContainerPort int
	Protocol      string // "tcp" or "udp"
	HostIP        string // optional; default 0.0.0.0
	HostPort      int    // 0 for dynamic assignment
}

// RunContainerOptions captures the container create/start parameters we need.
type RunContainerOptions struct {
	Name        string
	Hostname    string
	Image       string
	Args        []string
	Env         EnvVars
	Labels      map[string]string
	Mounts      Mounts
	Tmpfs       map[string]string // path -> options
	SecurityOpt []string
	Privileged  bool
	AutoRemove  bool
	Publish     []PortSpec
	// Network is the name of the user-defined network to attach this container to.
	// If empty, the runtime default network is used.
	Network string
}

// Mount describes a container mount
type Mount struct {
	// Type: "bind" | "volume" | "tmpfs"
	Type   string
	Source string
	Target string
	// Options string slice like ["ro", "z"]. Docker will join; Podman uses OCI.
	Options []string
}

// Mounts is a convenience slice type for helper methods
type Mounts []Mount

// ToBinds converts mounts to Docker bind strings: supports both bind and named volumes
func (ms Mounts) ToBinds() []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		t := strings.ToLower(m.Type)
		if t != "bind" && t != "volume" {
			continue
		}
		entry := m.Source + ":" + m.Target
		if len(m.Options) > 0 {
			entry = entry + ":" + strings.Join(m.Options, ",")
		}
		out = append(out, entry)
	}
	return out
}

// EnvVar represents a single environment variable
type EnvVar struct {
	Name  string
	Value string
}

// EnvVars is a slice of EnvVar with helpers for different backends
type EnvVars []EnvVar

// ToOSStrings converts env vars to ["KEY=VALUE", ...] form
func (ev EnvVars) ToOSStrings() []string {
	if len(ev) == 0 {
		return nil
	}
	out := make([]string, 0, len(ev))
	for _, e := range ev {
		out = append(out, e.Name+"="+e.Value)
	}
	return out
}

// ToMap converts env vars to map form
func (ev EnvVars) ToMap() map[string]string {
	if len(ev) == 0 {
		return nil
	}
	m := make(map[string]string, len(ev))
	for _, e := range ev {
		m[e.Name] = e.Value
	}
	return m
}

// ContainerInfo is a reduced view for listing clusters.
type ContainerInfo struct {
	ID      string
	Name    string
	Image   string
	Status  string
	Ports   string // human readable, e.g., "0.0.0.0:55131->6443/tcp"
	Created int64  // unix seconds
	Labels  map[string]string
}

// Runtime is the interface implemented by container runtimes.
type Runtime interface {
	Name() string

	RunContainer(ctx context.Context, opts RunContainerOptions) (string, error)
	ContainerExists(ctx context.Context, name string) (bool, error)
	ContainerIsRunning(ctx context.Context, name string) (bool, error)
	StopContainer(ctx context.Context, name string) error
	RemoveContainer(ctx context.Context, name string) error

	ExecInContainer(ctx context.Context, name string, command []string) (stdout string, exitCode int, err error)
	GetPortMapping(ctx context.Context, name string, containerPort int, protocol string) (hostIP string, hostPort int, err error)

	VolumeExists(ctx context.Context, name string) (bool, error)
	RemoveVolume(ctx context.Context, name string) error

	ListContainersByLabel(ctx context.Context, labelSelector map[string]string, includeStopped bool) ([]ContainerInfo, error)

	// CopyToContainer copies a local host path into the container at dstPath
	CopyToContainer(ctx context.Context, name string, srcPath string, dstPath string) error

	// SaveImageToTar saves a local image from the host runtime into a tar file at tarPath
	SaveImageToTar(ctx context.Context, imageRef string, tarPath string) error

	// EnsureNetwork ensures a user-defined network with the given name exists.
	// It should be idempotent.
	EnsureNetwork(ctx context.Context, name string) error
}

// Factory constructs a Runtime given a socket URI (may be empty for default).
type Factory func(ctx context.Context, socket string) (Runtime, error)

var registry = map[string]Factory{}

// Register adds a backend factory under a name, e.g. "docker", "podman".
func Register(name string, factory Factory) {
	registry[name] = factory
}

// GetFactory returns a registered backend factory by name.
func GetFactory(name string) (Factory, bool) {
	f, ok := registry[name]
	return f, ok
}

// Registered returns the list of registered backend names.
func Registered() []string {
	names := make([]string, 0, len(registry))
	for k := range registry {
		names = append(names, k)
	}
	return names
}
