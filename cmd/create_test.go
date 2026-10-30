package cmd

import (
	"os"
	"path/filepath"
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

func TestNodeMounts(t *testing.T) {
	cc := &config.ClusterConfig{
		SourcePath: "/repo/hack/cluster.yaml",
		Spec:       config.Spec{K0s: config.K0sSpec{Containerd: []string{"runtime.toml"}}},
	}
	node := &config.NodeSpec{Mounts: []config.Mount{
		{Type: "bind", Source: "../bin/shim", Target: "/usr/local/bin/shim", Options: []string{"ro"}},
		{Type: "bind", Source: "/abs/data", Target: "/data"},
		{Type: "volume", Source: "cache", Target: "/cache"},
	}}
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, os.MkdirAll(cc.ContainerdDir("c1"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(cc.ContainerdDir("c1"), "000_runtime.toml"), nil, 0o644))
	got := nodeMounts(cc, "c1", node)
	require.Len(t, got, 4)
	assert.Equal(t, filepath.Join(cc.ContainerdDir("c1"), "000_runtime.toml"), got[0].Source)
	assert.Equal(t, "/etc/k0s/containerd.d/000_runtime.toml", got[0].Target, "drop-ins are bound file by file")
	assert.Equal(t, "/repo/bin/shim", got[1].Source, "relative bind source resolves against the config file")
	assert.Equal(t, "/abs/data", got[2].Source)
	assert.Equal(t, "cache", got[3].Source, "volume names are not paths")

	// A relative config path still yields absolute bind sources.
	cc.SourcePath = "hack/cluster.yaml"
	wd, _ := os.Getwd()
	assert.Equal(t, wd+"/bin/shim", nodeMounts(cc, "c1", node)[1].Source)

	// No drop-ins: no containerd.d mount (the image's own stays visible).
	cc.Spec.K0s.Containerd = nil
	assert.Len(t, nodeMounts(cc, "c1", nil), 0)
}

func TestGcrSpecFromFlag(t *testing.T) {
	if g := gcrSpecFromFlag("v0.0.1"); g.Version != "v0.0.1" || g.Binary != "" {
		t.Fatalf("a version: %+v", g)
	}
	if g := gcrSpecFromFlag("./dist/gcr"); g.Binary == "" || !filepath.IsAbs(g.Binary) {
		t.Fatalf("a path: %+v", g)
	}
}
