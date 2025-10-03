// Package config provides configuration management for k0da clusters.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/imdario/mergo"
	"gopkg.in/yaml.v3"

	"github.com/makhov/k0da/internal/plugins"
)

const (
	DefaultNetwork      = "k0da"
	DefaultK0sImageRepo = "quay.io/k0sproject/k0s"
)

// DefaultK0sVersion is the default k0s version tag used for images.
// It is intentionally a var so it can be overridden via -ldflags at build time.
// Example:
//
//	-X github.com/makhov/k0da/internal/config.DefaultK0sVersion=v1.33.4-k0s.0
var DefaultK0sVersion = "v1.33.3-k0s.0"

const (
	LabelCluster     = "k0da.cluster"
	LabelClusterName = "k0da.cluster.name"
	LabelClusterType = "k0da.cluster.type"
	LabelNodeName    = "k0da.node.name"
	LabelNodeRole    = "k0da.node.role"
)

// ClusterConfig is a kind-like local cluster config aligned with k0s family style.
// Supports one or more nodes (we currently run single-node but keep structure future-proof).
type ClusterConfig struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       Spec   `yaml:"spec"`
	// SourcePath is the filesystem path of the loaded config file (not serialized)
	SourcePath string `yaml:"-"`
}

type Spec struct {
	Nodes   []NodeSpec  `yaml:"nodes"`
	K0s     K0sSpec     `yaml:"k0s"`
	Options OptionsSpec `yaml:"options,omitempty"`
}

type OptionsSpec struct {
	Network string `yaml:"network,omitempty"` // bridge network name, if empty, default "k0da" network will be used
}

type NodeSpec struct {
	Name   string            `yaml:"name,omitempty"`
	Role   string            `yaml:"role"` // controller|worker (currently only controller supported)
	Image  string            `yaml:"image,omitempty"`
	Args   []string          `yaml:"args,omitempty"`
	Ports  []Port            `yaml:"ports,omitempty"`
	Mounts []Mount           `yaml:"mounts,omitempty"`
	Env    map[string]string `yaml:"env,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

type Port struct {
	ContainerPort int    `yaml:"containerPort"`
	Protocol      string `yaml:"protocol,omitempty"`
	HostIP        string `yaml:"hostIP,omitempty"`
	HostPort      int    `yaml:"hostPort,omitempty"`
}

type Mount struct {
	Type    string   `yaml:"type"`
	Source  string   `yaml:"source"`
	Target  string   `yaml:"target"`
	Options []string `yaml:"options,omitempty"`
}

type K0sSpec struct {
	Image     string         `yaml:"image,omitempty"`
	Version   string         `yaml:"version,omitempty"`
	Config    map[string]any `yaml:"config,omitempty"`
	Args      []string       `yaml:"args,omitempty"`
	Manifests []string       `yaml:"manifests,omitempty"`
}

// LoadClusterConfig loads a cluster config from the given path.
// If path is empty, returns a default config.
// Always returns a valid config with validation applied.
func LoadClusterConfig(path string) (*ClusterConfig, error) {
	var c ClusterConfig

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read cluster config: %w", err)
		}
		if err := yaml.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("parse cluster config: %w", err)
		}
		// Remember the source path for resolving relative references (e.g., manifests)
		c.SourcePath = path
	}

	// Extract embedded plugins and add them to manifests
	pluginPaths, err := plugins.PluginManifestList()
	if err != nil {
		return nil, fmt.Errorf("failed to list plugins: %w", err)
	}

	// Add plugin manifests to the config
	c.Spec.K0s.Manifests = append(c.Spec.K0s.Manifests, pluginPaths...)

	// Apply defaults and validate
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid cluster config: %w", err)
	}

	return &c, nil
}

func (c *ClusterConfig) Validate() error {
	// Set defaults for empty configs
	if c.Kind == "" {
		c.Kind = "Cluster"
	}
	if c.APIVersion == "" {
		c.APIVersion = "k0da.k0sproject.io/v1alpha1"
	}

	// Validate kind
	if c.Kind != "Cluster" {
		return fmt.Errorf("unsupported kind: %q (expected Cluster)", c.Kind)
	}
	// apiVersion is informational for now; accept empty or v1alpha1
	if c.APIVersion != "k0da.k0sproject.io/v1alpha1" {
		return fmt.Errorf("unsupported apiVersion: %q", c.APIVersion)
	}
	if c.Spec.K0s.Image != "" && len(c.Spec.K0s.Image) < 3 {
		return fmt.Errorf("invalid k0s.image")
	}
	for _, n := range c.Spec.Nodes {
		if n.Role == "" {
			return fmt.Errorf("node role is required")
		}
	}
	if c.Spec.Options.Network == "" {
		c.Spec.Options.Network = DefaultNetwork
	}

	return nil
}

// PickPrimaryNode returns the controller node if present, otherwise the first node.
func (c *ClusterConfig) PickPrimaryNode() *NodeSpec {
	if c == nil {
		return nil
	}
	for i := range c.Spec.Nodes {
		if c.Spec.Nodes[i].Role == "controller" {
			return &c.Spec.Nodes[i]
		}
	}
	if len(c.Spec.Nodes) > 0 {
		return &c.Spec.Nodes[0]
	}
	return nil
}

// MaybeWriteInlineK0sConfig writes inline k0s config to a file under dir and returns its path.
func (c *ClusterConfig) MaybeWriteInlineK0sConfig(dir string) (string, error) {
	if c == nil || len(c.Spec.K0s.Config) == 0 {
		return "", nil
	}
	data, err := yaml.Marshal(c.Spec.K0s.Config)
	if err != nil {
		return "", fmt.Errorf("marshal inline k0s config: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}
	p := dir + "/k0s.yaml"
	if err := os.WriteFile(p, data, 0644); err != nil {
		return "", fmt.Errorf("write k0s config: %w", err)
	}
	return p, nil
}

func (c *ClusterConfig) ClusterDir(clusterName string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".k0da", "clusters", clusterName)
}

func (c *ClusterConfig) ConfigDir(clusterName string) string {
	return filepath.Join(c.ClusterDir(clusterName), "etc-k0s")
}

func (c *ClusterConfig) ConfigPath(clusterName string) string {
	return filepath.Join(c.ConfigDir(clusterName), "k0s.yaml")
}

func (c *ClusterConfig) ManifestDir(clusterName string) string {
	return filepath.Join(c.ClusterDir(clusterName), "manifests")
}

// EffectiveImage returns the k0s image to use based on precedence:
// 1) explicit image
// 2) DefaultK0sImageRepo + ":" + version
// 3) DefaultK0sImageRepo + ":" + DefaultK0sVersion
func (k K0sSpec) EffectiveImage() string {
	if k.Image != "" {
		return NormalizeImageTag(k.Image)
	}
	if k.Version != "" {
		return DefaultK0sImageRepo + ":" + NormalizeVersionTag(k.Version)
	}
	return DefaultK0sImageRepo + ":" + NormalizeVersionTag(DefaultK0sVersion)
}

// DefaultK0sConfig returns a minimal default k0s cluster configuration.
func DefaultK0sConfig() map[string]any {
	return map[string]any{
		"apiVersion": "k0s.k0sproject.io/v1beta1",
		"kind":       "ClusterConfig",
		"metadata":   map[string]string{"name": "k0s", "namespace": "kube-system"},
		"spec":       map[string]any{},
	}
}

// EffectiveK0sConfig returns the merged k0s config: defaults overlaid with user-specified values.
func (c *ClusterConfig) EffectiveK0sConfig() map[string]any {
	base := DefaultK0sConfig()
	if c == nil || len(c.Spec.K0s.Config) == 0 {
		return base
	}
	// Merge user config into defaults; user values override defaults
	spec, ok := c.Spec.K0s.Config["spec"]
	if !ok {
		return base
	}
	baseSpec := base["spec"].(map[string]any)
	if err := mergo.Merge(&baseSpec, spec.(map[string]any), mergo.WithOverride); err != nil {
		// Fallback to internal deep merge on error
		panic(fmt.Errorf("merge k0s config: %w", err))
	}
	base["spec"] = baseSpec
	return base
}

// WriteEffectiveK0sConfig writes the effective k0s config (defaults merged with inline user config) to dir.
func (c *ClusterConfig) WriteEffectiveK0sConfig(clusterName string) error {
	dir := c.ConfigDir(clusterName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	data, err := yaml.Marshal(c.EffectiveK0sConfig())
	if err != nil {
		return fmt.Errorf("marshal k0s config: %w", err)
	}
	if err := os.WriteFile(c.ConfigPath(clusterName), data, 0644); err != nil {
		return fmt.Errorf("write k0s config: %w", err)
	}
	return nil
}
