package cmd

import (
	"os"
	"testing"

	"github.com/makhov/k0da/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildK0sControllerArgs(t *testing.T) {
	tests := []struct {
		name      string
		cc        *config.ClusterConfig
		node      *config.NodeSpec
		isPrimary bool
		expected  []string
	}{
		{
			name: "primary single node",
			cc: &config.ClusterConfig{
				Spec: config.Spec{
					Nodes: []config.NodeSpec{{Name: "node1", Role: "controller"}},
					K0s:   config.K0sSpec{},
				},
			},
			node:      &config.NodeSpec{Name: "node1", Role: "controller"},
			isPrimary: true,
			expected: []string{
				"k0s", "controller",
				"--enable-dynamic-config", "--disable-components=metrics-server", "--ignore-pre-flight-checks",
				"--single", "--config", "/etc/k0s/k0s.yaml",
			},
		},
		{
			name: "primary multi node",
			cc: &config.ClusterConfig{
				Spec: config.Spec{
					Nodes: []config.NodeSpec{
						{Name: "node1", Role: "controller"},
						{Name: "node2", Role: "controller"},
					},
					K0s: config.K0sSpec{},
				},
			},
			node:      &config.NodeSpec{Name: "node1", Role: "controller"},
			isPrimary: true,
			expected: []string{
				"k0s", "controller",
				"--enable-dynamic-config", "--disable-components=metrics-server", "--ignore-pre-flight-checks",
				"--enable-worker", "--no-taints",
				"--config", "/etc/k0s/k0s.yaml",
			},
		},
		{
			name: "secondary controller node",
			cc: &config.ClusterConfig{
				Spec: config.Spec{
					Nodes: []config.NodeSpec{
						{Name: "node1", Role: "controller"},
						{Name: "node2", Role: "controller"},
					},
					K0s: config.K0sSpec{},
				},
			},
			node:      &config.NodeSpec{Name: "node2", Role: "controller"},
			isPrimary: false,
			expected: []string{
				"k0s", "controller",
				"--enable-dynamic-config", "--disable-components=metrics-server", "--ignore-pre-flight-checks",
				"--enable-worker", "--no-taints",
				"--token-file", "/etc/k0s/join.token",
				"--config", "/etc/k0s/k0s.yaml",
			},
		},
		{
			name: "with global k0s args",
			cc: &config.ClusterConfig{
				Spec: config.Spec{
					Nodes: []config.NodeSpec{{Name: "node1", Role: "controller"}},
					K0s: config.K0sSpec{
						Args: []string{"--debug", "--data-dir=/custom/data"},
					},
				},
			},
			node:      &config.NodeSpec{Name: "node1", Role: "controller"},
			isPrimary: true,
			expected: []string{
				"k0s", "controller",
				"--enable-dynamic-config", "--disable-components=metrics-server", "--ignore-pre-flight-checks",
				"--single",
				"--config", "/etc/k0s/k0s.yaml",
				"--debug", "--data-dir=/custom/data",
			},
		},
		{
			name: "with node-specific args",
			cc: &config.ClusterConfig{
				Spec: config.Spec{
					Nodes: []config.NodeSpec{{Name: "node1", Role: "controller"}},
					K0s:   config.K0sSpec{},
				},
			},
			node: &config.NodeSpec{
				Name: "node1",
				Role: "controller",
				Args: []string{"--custom-arg=value", "--another-arg"},
			},
			isPrimary: true,
			expected: []string{
				"k0s", "controller",
				"--enable-dynamic-config", "--disable-components=metrics-server", "--ignore-pre-flight-checks",
				"--single",
				"--config", "/etc/k0s/k0s.yaml",
				"--custom-arg=value", "--another-arg",
			},
		},
		{
			name: "with both global and node-specific args",
			cc: &config.ClusterConfig{
				Spec: config.Spec{
					Nodes: []config.NodeSpec{{Name: "node1", Role: "controller"}},
					K0s: config.K0sSpec{
						Args: []string{"--global-arg=value"},
					},
				},
			},
			node: &config.NodeSpec{
				Name: "node1",
				Role: "controller",
				Args: []string{"--node-arg=value"},
			},
			isPrimary: true,
			expected: []string{
				"k0s", "controller",
				"--enable-dynamic-config", "--disable-components=metrics-server", "--ignore-pre-flight-checks",
				"--single",
				"--config", "/etc/k0s/k0s.yaml",
				"--global-arg=value",
				"--node-arg=value",
			},
		},
		{
			name: "secondary controller with args",
			cc: &config.ClusterConfig{
				Spec: config.Spec{
					Nodes: []config.NodeSpec{
						{Name: "node1", Role: "controller"},
						{Name: "node2", Role: "controller"},
					},
					K0s: config.K0sSpec{
						Args: []string{"--global-arg=value"},
					},
				},
			},
			node: &config.NodeSpec{
				Name: "node2",
				Role: "controller",
				Args: []string{"--node-arg=value"},
			},
			isPrimary: false,
			expected: []string{
				"k0s", "controller",
				"--enable-dynamic-config", "--disable-components=metrics-server", "--ignore-pre-flight-checks",
				"--enable-worker", "--no-taints",
				"--token-file", "/etc/k0s/join.token",
				"--config", "/etc/k0s/k0s.yaml",
				"--global-arg=value",
				"--node-arg=value",
			},
		},
		{
			name: "nil node",
			cc: &config.ClusterConfig{
				Spec: config.Spec{
					Nodes: []config.NodeSpec{{Name: "node1", Role: "controller"}},
					K0s:   config.K0sSpec{},
				},
			},
			node:      nil,
			isPrimary: true,
			expected: []string{
				"k0s", "controller",
				"--enable-dynamic-config", "--disable-components=metrics-server", "--ignore-pre-flight-checks",
				"--single",
				"--config", "/etc/k0s/k0s.yaml",
			},
		},
		{
			name: "empty global and node args",
			cc: &config.ClusterConfig{
				Spec: config.Spec{
					Nodes: []config.NodeSpec{{Name: "node1", Role: "controller"}},
					K0s: config.K0sSpec{
						Args: []string{},
					},
				},
			},
			node: &config.NodeSpec{
				Name: "node1",
				Role: "controller",
				Args: []string{},
			},
			isPrimary: true,
			expected: []string{
				"k0s", "controller",
				"--enable-dynamic-config", "--disable-components=metrics-server", "--ignore-pre-flight-checks",
				"--single",
				"--config", "/etc/k0s/k0s.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildK0sControllerArgs(tt.cc, tt.node, tt.isPrimary)
			assert.Equal(t, tt.expected, result, "buildK0sControllerArgs() = %v, want %v", result, tt.expected)
		})
	}
}

func TestRootlessNodeAdjustments(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cc := &config.ClusterConfig{}

	base := []string{"k0s", "controller", "--single"}
	args, mount, err := rootlessNodeAdjustments(cc, "c1", base)
	require.NoError(t, err)

	assert.Equal(t, config.RootlessKubeletArg, args[len(args)-1],
		"kubelet must be told it runs in a user namespace or it cannot open /dev/kmsg")
	assert.Equal(t, "/etc/k0s/containerd.d/rootless.toml", mount.Target,
		"k0s imports /etc/k0s/containerd.d/*.toml")
	assert.Equal(t, []string{"ro"}, mount.Options)

	// The drop-in must exist on the host and stop runc lowering oom_score_adj,
	// which otherwise fails every pod sandbox.
	data, err := os.ReadFile(mount.Source)
	require.NoError(t, err)
	assert.Contains(t, string(data), "restrict_oom_score_adj = true")
	assert.Contains(t, string(data), `[plugins."io.containerd.cri.v1.runtime"]`)
}

func TestRootlessNodeAdjustmentsRespectsExplicitKubeletArgs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cc := &config.ClusterConfig{}

	base := []string{"k0s", "controller", "--kubelet-extra-args=--v=4"}
	args, mount, err := rootlessNodeAdjustments(cc, "c1", base)
	require.NoError(t, err)

	assert.Equal(t, base, args, "must not pass --kubelet-extra-args twice")
	assert.NotEmpty(t, mount.Source, "containerd drop-in is still required")
}
