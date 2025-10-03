package utils

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	k0daconfig "github.com/makhov/k0da/internal/config"
	"github.com/makhov/k0da/internal/runtime"
)

// WaitForK0sReady waits for k0s to be ready in a container
func WaitForK0sReady(ctx context.Context, r runtime.Runtime, containerName, timeout string) error {
	fmt.Printf("Waiting for cluster to be ready (timeout: %s)...\n", timeout)

	// Parse timeout duration
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		timeoutDuration = 60 * time.Second // Default to 60 seconds
	}

	startTime := time.Now()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if k0s status is responding
			if isK0sReady(ctx, r, containerName) {
				fmt.Println("✅ k0s is ready!")
				return nil
			}

			// Check timeout
			if time.Since(startTime) > timeoutDuration {
				return fmt.Errorf("timeout waiting for cluster to be ready after %s", timeout)
			}

			fmt.Print(".")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// isK0sReady checks if k0s is ready in a container
func isK0sReady(ctx context.Context, r runtime.Runtime, containerName string) bool {
	stdout, exit, err := r.ExecInContainer(ctx, containerName, []string{"k0s", "status"})
	if err != nil || exit != 0 {
		return false
	}
	return strings.Contains(stdout, "Kube-api probing successful: true")
}

// AllocateHostPort reserves a free TCP port on the given host IP (defaults to 0.0.0.0).
// It opens a listener on hostIP:0, reads the assigned port, then closes the listener
// and returns the port number. Returns 0 if allocation failed.
func AllocateHostPort(hostIP string) (int, error) {
	hip := strings.TrimSpace(hostIP)
	if hip == "" {
		hip = "0.0.0.0"
	}
	ln, err := net.Listen("tcp", hip+":0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = ln.Close() }()
	if ta, ok := ln.Addr().(*net.TCPAddr); ok {
		return ta.Port, nil
	}
	return 0, fmt.Errorf("unable to determine allocated port")
}

// GetContainerPort gets the external port mapping for a container
func GetContainerPort(ctx context.Context, b runtime.Runtime, containerName string) (string, error) {
	// Retry a few times to allow backends to register dynamic port mappings
	var lastErr error
	for i := 0; i < 15; i++ { // up to ~15s
		_, hostPort, err := b.GetPortMapping(ctx, containerName, 6443, "tcp")
		if err == nil && hostPort != 0 {
			return fmt.Sprintf("%d", hostPort), nil
		}
		lastErr = err
		time.Sleep(1 * time.Second)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("port mapping not found after retries")
	}
	return "", lastErr
}

// Kubeconfig structures for proper parsing
type Kubeconfig struct {
	APIVersion     string                 `yaml:"apiVersion"`
	Kind           string                 `yaml:"kind"`
	Clusters       []NamedCluster         `yaml:"clusters"`
	Contexts       []NamedContext         `yaml:"contexts"`
	CurrentContext string                 `yaml:"current-context"`
	Users          []NamedUser            `yaml:"users"`
	Preferences    map[string]interface{} `yaml:"preferences,omitempty"`
}

type NamedCluster struct {
	Name    string  `yaml:"name"`
	Cluster Cluster `yaml:"cluster"`
}

type Cluster struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
}

type NamedContext struct {
	Name    string  `yaml:"name"`
	Context Context `yaml:"context"`
}

type Context struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

type NamedUser struct {
	Name string `yaml:"name"`
	User User   `yaml:"user"`
}

type User struct {
	ClientCertificateData string `yaml:"client-certificate-data"`
	ClientKeyData         string `yaml:"client-key-data"`
}

// LoadKubeconfig loads a kubeconfig from file
func LoadKubeconfig(filePath string) (*Kubeconfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read kubeconfig file: %w", err)
	}

	var kubeconfig Kubeconfig
	if err := yaml.Unmarshal(data, &kubeconfig); err != nil {
		return nil, fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	return &kubeconfig, nil
}

// MarshalKubeconfig marshals a kubeconfig to YAML bytes
func MarshalKubeconfig(kubeconfig *Kubeconfig) ([]byte, error) {
	data, err := yaml.Marshal(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}
	return data, nil
}

// SaveKubeconfig saves a kubeconfig to file
func SaveKubeconfig(kubeconfig *Kubeconfig, filePath string) error {
	data, err := MarshalKubeconfig(kubeconfig)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

// CopyManifestsToDir copies provided manifest file paths into destination directory.
// Paths are resolved relative to baseDir when not absolute. Files are written
// into destDir with a numeric prefix to preserve ordering when provided.
func CopyManifestsToDir(cc *k0daconfig.ClusterConfig, destDir string) error {
	if cc == nil || len(cc.Spec.K0s.Manifests) == 0 {
		return nil
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create manifests directory: %w", err)
	}
	if err := RemoveAllFiles(destDir); err != nil {
		return fmt.Errorf("failed to clean manifests dir: %w", err)
	}

	baseDir := ""
	if strings.TrimSpace(cc.SourcePath) != "" {
		baseDir = filepath.Dir(cc.SourcePath)
	}

	return copyManifestsToDir(cc.Spec.K0s.Manifests, baseDir, destDir)
}

func isURL(str string) bool {
	u, err := url.Parse(str)
	if err != nil {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "http"
}

func urlBase(str string) string {
	u, _ := url.Parse(str)
	return path.Base(u.Path)
}

func copyManifestsToDir(paths []string, baseDir string, destDir string) error {
	for i, mp := range paths {
		p := strings.TrimSpace(mp)
		if p == "" {
			continue
		}
		var (
			data     []byte
			baseName string
			err      error
		)
		if isURL(p) {
			resp, err := http.Get(p)
			if err != nil {
				return err
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("bad status: %s", resp.Status)
			}

			data, err = io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read response for file %s body: %w", p, err)
			}
			baseName = urlBase(p)
		} else {
			abs := p
			if !filepath.IsAbs(p) && strings.TrimSpace(baseDir) != "" {
				abs = filepath.Join(baseDir, p)
			}
			data, err = os.ReadFile(abs)
			if err != nil {
				return fmt.Errorf("failed to read manifest %q: %w", p, err)
			}
			baseName = filepath.Base(abs)
		}
		// Prefix with index to keep deterministic order
		dst := filepath.Join(destDir, fmt.Sprintf("%03d_%s", i, baseName))
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("failed to write manifest to %q: %w", dst, err)
		}
	}
	return nil
}

// RemoveAllFiles removes all regular files in the given directory (non-recursive).
func RemoveAllFiles(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.Type().IsRegular() || (e.Type() == fs.ModeSymlink) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
			continue
		}
		// Also remove nested files if any leftover directories exist
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
	return nil
}

// defaultKubeconfigPath returns the path to the default kubeconfig file
// (first entry of KUBECONFIG if set, otherwise $HOME/.kube/config)
func defaultKubeconfigPath() string {
	if v := strings.TrimSpace(os.Getenv("KUBECONFIG")); v != "" {
		parts := strings.Split(v, string(os.PathListSeparator))
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			return parts[0]
		}
	}
	home := os.Getenv("HOME")
	if strings.TrimSpace(home) == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	return filepath.Join(home, ".kube", "config")
}

// AddClusterToKubeconfig adds a new cluster to the default kubeconfig
func AddClusterToKubeconfig(ctx context.Context, b runtime.Runtime, clusterName, containerName string) error {
	// Get the original kubeconfig from the container
	stdout, exit, err := b.ExecInContainer(ctx, containerName, []string{"k0s", "kubeconfig", "admin"})
	if err != nil || exit != 0 {
		return fmt.Errorf("failed to get kubeconfig from container: %v", err)
	}

	// Parse the container kubeconfig
	var containerKubeconfig Kubeconfig
	if err := yaml.Unmarshal([]byte(stdout), &containerKubeconfig); err != nil {
		return fmt.Errorf("failed to parse container kubeconfig: %w", err)
	}

	// Get the port mapping for the container
	port, err := GetContainerPort(ctx, b, containerName)
	if err != nil {
		return fmt.Errorf("failed to get container port: %w", err)
	}

	// Update the server URL with correct host and port
	if len(containerKubeconfig.Clusters) > 0 {
		containerKubeconfig.Clusters[0].Cluster.Server = fmt.Sprintf("https://127.0.0.1:%s", port)
	}

	// Load or create the default kubeconfig
	kubeconfigPath := defaultKubeconfigPath()
	var kc *Kubeconfig
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		kc = &Kubeconfig{
			APIVersion:     "v1",
			Kind:           "Config",
			Clusters:       []NamedCluster{},
			Contexts:       []NamedContext{},
			CurrentContext: "",
			Users:          []NamedUser{},
			Preferences:    make(map[string]interface{}),
		}
	} else {
		kc, err = LoadKubeconfig(kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}

	// Remove existing cluster/context/user with same names
	kc = removeClusterFromKubeconfig(kc, clusterName)

	clusterNameFormatted := fmt.Sprintf("k0da-%s", clusterName)
	contextNameFormatted := fmt.Sprintf("k0da-%s", clusterName)
	userNameFormatted := fmt.Sprintf("k0da-%s", clusterName)

	// Add cluster
	if len(containerKubeconfig.Clusters) > 0 {
		kc.Clusters = append(kc.Clusters, NamedCluster{
			Name:    clusterNameFormatted,
			Cluster: containerKubeconfig.Clusters[0].Cluster,
		})
	}
	// Add context
	if len(containerKubeconfig.Contexts) > 0 {
		kc.Contexts = append(kc.Contexts, NamedContext{
			Name: contextNameFormatted,
			Context: Context{
				Cluster: clusterNameFormatted,
				User:    userNameFormatted,
			},
		})
	}
	// Add user
	if len(containerKubeconfig.Users) > 0 {
		kc.Users = append(kc.Users, NamedUser{
			Name: userNameFormatted,
			User: containerKubeconfig.Users[0].User,
		})
	}

	// Set as current context
	kc.CurrentContext = contextNameFormatted

	// Save kubeconfig
	if err := SaveKubeconfig(kc, kubeconfigPath); err != nil {
		return fmt.Errorf("failed to save kubeconfig: %w", err)
	}

	return nil
}

// RemoveClusterFromKubeconfig removes a cluster from the default kubeconfig
func RemoveClusterFromKubeconfig(clusterName string) error {
	kubeconfigPath := defaultKubeconfigPath()

	var kc *Kubeconfig
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return nil
	}
	var err error
	kc, err = LoadKubeconfig(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Remove the cluster/context/user
	kc = removeClusterFromKubeconfig(kc, clusterName)

	// If current context was removed, set to first available context (if any)
	if kc.CurrentContext == fmt.Sprintf("k0da-%s", clusterName) {
		if len(kc.Contexts) > 0 {
			kc.CurrentContext = kc.Contexts[0].Name
		} else {
			kc.CurrentContext = ""
		}
	}

	// Save the updated kubeconfig (do not delete the file even if empty)
	if err := SaveKubeconfig(kc, kubeconfigPath); err != nil {
		return fmt.Errorf("failed to save kubeconfig: %w", err)
	}

	return nil
}

// removeClusterFromKubeconfig is a helper function to remove a cluster from kubeconfig
func removeClusterFromKubeconfig(kubeconfig *Kubeconfig, clusterName string) *Kubeconfig {
	clusterNameFormatted := fmt.Sprintf("k0da-%s", clusterName)
	contextNameFormatted := fmt.Sprintf("k0da-%s", clusterName)
	userNameFormatted := fmt.Sprintf("k0da-%s", clusterName)

	// Remove cluster
	var newClusters []NamedCluster
	for _, cluster := range kubeconfig.Clusters {
		if cluster.Name != clusterNameFormatted {
			newClusters = append(newClusters, cluster)
		}
	}
	kubeconfig.Clusters = newClusters

	// Remove context
	var newContexts []NamedContext
	for _, context := range kubeconfig.Contexts {
		if context.Name != contextNameFormatted {
			newContexts = append(newContexts, context)
		}
	}
	kubeconfig.Contexts = newContexts

	// Remove user
	var newUsers []NamedUser
	for _, user := range kubeconfig.Users {
		if user.Name != userNameFormatted {
			newUsers = append(newUsers, user)
		}
	}
	kubeconfig.Users = newUsers

	return kubeconfig
}
