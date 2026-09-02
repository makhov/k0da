package utils

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/makhov/k0da/internal/runtime"
	"github.com/stretchr/testify/require"
)

// fakeRuntime implements runtime.Runtime for tests
type fakeRuntime struct {
	execStdout   string
	execExitCode int
	execErr      error

	// execFunc, when set, takes precedence and lets a test answer per command.
	execFunc func(args []string) (string, int, error)

	portIP  string
	port    int
	portErr error

	rootless bool
}

func (f *fakeRuntime) Name() string { return "fake" }
func (f *fakeRuntime) RunContainer(_ context.Context, _ runtime.RunContainerOptions) (string, error) {
	return "", nil
}
func (f *fakeRuntime) ContainerExists(_ context.Context, _ string) (bool, error) {
	return true, nil
}
func (f *fakeRuntime) ContainerIsRunning(_ context.Context, _ string) (bool, error) {
	return true, nil
}
func (f *fakeRuntime) StopContainer(_ context.Context, _ string) error   { return nil }
func (f *fakeRuntime) RemoveContainer(_ context.Context, _ string) error { return nil }
func (f *fakeRuntime) ExecInContainer(_ context.Context, _ string, args []string) (string, int, error) {
	if f.execFunc != nil {
		return f.execFunc(args)
	}
	return f.execStdout, f.execExitCode, f.execErr
}
func (f *fakeRuntime) GetPortMapping(_ context.Context, _ string, _ int, _ string) (string, int, error) {
	return f.portIP, f.port, f.portErr
}
func (f *fakeRuntime) VolumeExists(_ context.Context, _ string) (bool, error) { return false, nil }
func (f *fakeRuntime) RemoveVolume(_ context.Context, _ string) error         { return nil }
func (f *fakeRuntime) ListContainersByLabel(_ context.Context, _ map[string]string, _ bool) ([]runtime.ContainerInfo, error) {
	return nil, nil
}
func (f *fakeRuntime) CopyToContainer(_ context.Context, _ string, _ string, _ string) error {
	return nil
}
func (f *fakeRuntime) SaveImageToTar(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *fakeRuntime) EnsureNetwork(_ context.Context, _ string) error { return nil }

func (f *fakeRuntime) IsRootless(_ context.Context) (bool, error) { return f.rootless, nil }

func TestWaitForK0sReady_SucceedsImmediately(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	r := &fakeRuntime{
		execStdout:   "Kube-api probing successful: true\n",
		execExitCode: 0,
	}

	err := WaitForK0sReady(ctx, r, "test", "2s")
	require.NoError(t, err)
}

func TestAddAndRemoveClusterToUnifiedKubeconfig(t *testing.T) {
	// Isolated HOME
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	adminKubeconfigYAML := `apiVersion: v1
kind: Config
clusters:
- name: k0s-admin
  cluster:
    server: https://127.0.0.1:6443
    certificate-authority-data: Cg==
contexts:
- name: admin@k0s
  context:
    cluster: k0s-admin
    user: k0s-admin
users:
- name: k0s-admin
  user:
    client-certificate-data: Cg==
    client-key-data: Cg==
`

	r := &fakeRuntime{
		execStdout:   adminKubeconfigYAML,
		execExitCode: 0,
		portIP:       "0.0.0.0",
		port:         52345,
	}

	ctx := context.Background()
	err := AddClusterToKubeconfig(ctx, r, "test", "test")
	require.NoError(t, err)

	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".kube", "config")
	kc, err := LoadKubeconfig(path)
	require.NoError(t, err)

	require.Equal(t, "k0da-test", kc.CurrentContext)
	require.Len(t, kc.Clusters, 1)
	require.Equal(t, "k0da-test", kc.Clusters[0].Name)
	require.Equal(t, "https://127.0.0.1:52345", kc.Clusters[0].Cluster.Server)
}

func TestGetContainerPort(t *testing.T) {
	r := &fakeRuntime{portIP: "0.0.0.0", port: 60000}
	port, err := GetContainerPort(context.Background(), r, "any")
	require.NoError(t, err)
	require.Equal(t, "60000", port)
}

// Regression: kube-api answers before containerd is up. Reporting the cluster
// ready at that point makes `k0da load` fail with
// "cannot access socket /run/k0s/containerd.sock".
func TestWaitForK0sReady_WaitsForContainerd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	r := &fakeRuntime{
		execFunc: func(args []string) (string, int, error) {
			if len(args) > 1 && args[1] == "ctr" {
				return "Error: cannot access socket /run/k0s/containerd.sock", 1, nil
			}
			return "Kube-api probing successful: true\n", 0, nil
		},
	}

	err := WaitForK0sReady(ctx, r, "test", "1s")
	require.Error(t, err)
	require.Contains(t, err.Error(), "timeout waiting for cluster to be ready")
}

func TestWaitForK0sReady_SucceedsOnceContainerdIsUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var ctrCalls int
	r := &fakeRuntime{
		execFunc: func(args []string) (string, int, error) {
			if len(args) > 1 && args[1] == "ctr" {
				ctrCalls++
				if ctrCalls < 2 {
					return "Error: cannot access socket /run/k0s/containerd.sock", 1, nil
				}
				return "Server:\n  Version: 2.3.4", 0, nil
			}
			return "Kube-api probing successful: true\n", 0, nil
		},
	}

	require.NoError(t, WaitForK0sReady(ctx, r, "test", "10s"))
}
