package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/makhov/k0da/internal/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// kubeconfigRuntime is a cluster's controller as printKubeconfig sees it;
// the rest of runtime.Runtime is not called.
type kubeconfigRuntime struct {
	runtime.Runtime
	running bool
}

func (r *kubeconfigRuntime) ContainerIsRunning(_ context.Context, _ string) (bool, error) {
	return r.running, nil
}

func (r *kubeconfigRuntime) ExecInContainer(_ context.Context, _ string, _ []string) (string, int, error) {
	return `apiVersion: v1
kind: Config
clusters:
- name: k0s-admin
  cluster:
    server: https://172.18.0.2:6443
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
`, 0, nil
}

func (r *kubeconfigRuntime) GetPortMapping(_ context.Context, _ string, _ int, _ string) (string, int, error) {
	return "0.0.0.0", 52345, nil
}

func TestPrintKubeconfig(t *testing.T) {
	// No kubeconfig file anywhere: it comes from the controller.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KUBECONFIG", "")

	var out bytes.Buffer
	require.NoError(t, printKubeconfig(context.Background(), &kubeconfigRuntime{running: true}, "doom", &out))
	s := out.String()
	assert.Contains(t, s, "current-context: k0da-doom")
	assert.Contains(t, s, "name: k0da-doom")
	assert.Contains(t, s, "server: https://127.0.0.1:52345")
	assert.Contains(t, s, "client-key-data: Cg==")
}

func TestPrintKubeconfigClusterNotFound(t *testing.T) {
	err := printKubeconfig(context.Background(), &kubeconfigRuntime{}, "nope", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster 'nope' not found")
}
