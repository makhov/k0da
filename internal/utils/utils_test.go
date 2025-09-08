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

	portIP  string
	port    int
	portErr error
}

func (f *fakeRuntime) Name() string { return "fake" }
func (f *fakeRuntime) RunContainer(ctx context.Context, opts runtime.RunContainerOptions) (string, error) {
	return "", nil
}
func (f *fakeRuntime) ContainerExists(ctx context.Context, name string) (bool, error) {
	return true, nil
}
func (f *fakeRuntime) ContainerIsRunning(ctx context.Context, name string) (bool, error) {
	return true, nil
}
func (f *fakeRuntime) StopContainer(ctx context.Context, name string) error   { return nil }
func (f *fakeRuntime) RemoveContainer(ctx context.Context, name string) error { return nil }
func (f *fakeRuntime) ExecInContainer(ctx context.Context, name string, command []string) (string, int, error) {
	return f.execStdout, f.execExitCode, f.execErr
}
func (f *fakeRuntime) GetPortMapping(ctx context.Context, name string, containerPort int, protocol string) (string, int, error) {
	return f.portIP, f.port, f.portErr
}
func (f *fakeRuntime) VolumeExists(ctx context.Context, name string) (bool, error) { return false, nil }
func (f *fakeRuntime) RemoveVolume(ctx context.Context, name string) error         { return nil }
func (f *fakeRuntime) ListContainersByLabel(ctx context.Context, labelSelector map[string]string, includeStopped bool) ([]runtime.ContainerInfo, error) {
	return nil, nil
}
func (f *fakeRuntime) CopyToContainer(ctx context.Context, name string, srcPath string, dstPath string) error {
	return nil
}
func (f *fakeRuntime) SaveImageToTar(ctx context.Context, imageRef string, tarPath string) error {
	return nil
}

func (f *fakeRuntime) EnsureNetwork(ctx context.Context, name string) error { return nil }

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
