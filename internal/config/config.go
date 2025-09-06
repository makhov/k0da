package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	DefaultNetwork      = "k0da"
	DefaultK0sVersion   = "v1.33.3-k0s.0"
	DefaultK0sImageRepo = "quay.io/k0sproject/k0s"
)

// ClusterConfig is a kind-like local cluster config aligned with k0s family style.
// Supports one or more nodes (we currently run single-node but keep structure future-proof).
type ClusterConfig struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Spec       Spec   `yaml:"spec"`
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
	Image   string         `yaml:"image,omitempty"`
	Version string         `yaml:"version,omitempty"`
	Config  map[string]any `yaml:"config,omitempty"`
	Args    []string       `yaml:"args,omitempty"`
}

func LoadClusterConfig(path string) (*ClusterConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cluster config: %w", err)
	}
	var c ClusterConfig
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse cluster config: %w", err)
	}
	return &c, nil
}

func (c *ClusterConfig) Validate() error {
	if c.Kind != "Cluster" {
		return fmt.Errorf("unsupported kind: %q (expected Cluster)", c.Kind)
	}
	// apiVersion is informational for now; accept empty or v1alpha1
	if c.APIVersion != "" && c.APIVersion != "k0da.k0sproject.io/v1alpha1" {
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

// EffectiveImage returns the k0s image to use based on precedence:
// 1) explicit image
// 2) DefaultK0sImageRepo + ":" + version
// 3) DefaultK0sImageRepo + ":" + DefaultK0sVersion
func (k K0sSpec) EffectiveImage() string {
	if k.Image != "" {
		return k.Image
	}
	if k.Version != "" {
		return DefaultK0sImageRepo + ":" + k.Version
	}
	return DefaultK0sImageRepo + ":" + DefaultK0sVersion
}
