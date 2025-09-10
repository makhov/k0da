package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/makhov/k0da/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubeconfigCommand(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create the unified kubeconfig directory
	kubeconfigDir := filepath.Join(tempDir, ".k0da", "clusters")
	err := os.MkdirAll(kubeconfigDir, 0755)
	require.NoError(t, err)

	// Create a test unified kubeconfig
	unifiedKubeconfig := &utils.Kubeconfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "k0da-test-cluster",
		Clusters: []utils.NamedCluster{
			{
				Name: "k0da-test-cluster",
				Cluster: utils.Cluster{
					Server:                   "https://localhost:6443",
					CertificateAuthorityData: "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURBRENDQWVpZ0F3SUJBZ0lVSS93UE92",
				},
			},
		},
		Contexts: []utils.NamedContext{
			{
				Name: "k0da-test-cluster",
				Context: utils.Context{
					Cluster: "k0da-test-cluster",
					User:    "k0da-test-cluster",
				},
			},
		},
		Users: []utils.NamedUser{
			{
				Name: "k0da-test-cluster",
				User: utils.User{
					ClientCertificateData: "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURXVENDQWtHZ0F3SUJBZ0lVSHdINlRzTjh",
					ClientKeyData:         "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFcFFJQkFBS0NBUUVBMGxIMXZhL3QxZmNlSnh",
				},
			},
		},
	}

	// Save the unified kubeconfig
	unifiedKubeconfigPath := filepath.Join(kubeconfigDir, "kubeconfig")
	err = utils.SaveKubeconfig(unifiedKubeconfig, unifiedKubeconfigPath)
	require.NoError(t, err)

	// Test the kubeconfig command function directly
	kubeconfigClusterName = "test-cluster"

	// Capture output
	originalStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = runKubeconfig(kubeconfigCmd, []string{})
	w.Close()

	// Read output
	output := make([]byte, 1024)
	n, _ := r.Read(output)
	os.Stdout = originalStdout

	// Check that the command succeeded
	assert.NoError(t, err)

	// Check that the output contains expected kubeconfig elements
	outputStr := string(output[:n])
	assert.Contains(t, outputStr, "apiVersion: v1")
	assert.Contains(t, outputStr, "kind: Config")
	assert.Contains(t, outputStr, "name: k0da-test-cluster")
	assert.Contains(t, outputStr, "server: https://localhost:6443")
	assert.Contains(t, outputStr, "current-context: k0da-test-cluster")
}

func TestKubeconfigCommandClusterNotFound(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create the unified kubeconfig directory
	kubeconfigDir := filepath.Join(tempDir, ".k0da", "clusters")
	err := os.MkdirAll(kubeconfigDir, 0755)
	require.NoError(t, err)

	// Create an empty unified kubeconfig
	unifiedKubeconfig := &utils.Kubeconfig{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "",
		Clusters:       []utils.NamedCluster{},
		Contexts:       []utils.NamedContext{},
		Users:          []utils.NamedUser{},
	}

	// Save the unified kubeconfig
	unifiedKubeconfigPath := filepath.Join(kubeconfigDir, "kubeconfig")
	err = utils.SaveKubeconfig(unifiedKubeconfig, unifiedKubeconfigPath)
	require.NoError(t, err)

	// Test the kubeconfig command with non-existent cluster
	kubeconfigClusterName = "non-existent-cluster"

	err = runKubeconfig(kubeconfigCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cluster 'non-existent-cluster' not found")
}

func TestKubeconfigCommandNoUnifiedKubeconfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Test the kubeconfig command when no unified kubeconfig exists
	kubeconfigClusterName = "test-cluster"

	err := runKubeconfig(kubeconfigCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no unified kubeconfig found")
}
