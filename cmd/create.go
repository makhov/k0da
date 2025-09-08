package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	k0daconfig "github.com/makhov/k0da/internal/config"
	"github.com/makhov/k0da/internal/runtime"
	"github.com/makhov/k0da/internal/utils"
	"github.com/spf13/cobra"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create [cluster-name]",
	Short: "Create a new k0s cluster",
	Long: `Create a new k0s cluster with the specified name.
This command will set up a lightweight Kubernetes cluster using k0s distribution.
The cluster name can be provided as an argument or via the --name flag.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCreate,
}

var (
	clusterConfigPath string
	image             string
	wait              bool
	timeout           string
	name              string
)

func init() {
	rootCmd.AddCommand(createCmd)

	// Here you will define your flags and configuration settings.
	createCmd.Flags().StringVarP(&name, "name", "n", DefaultClusterName, "name of the cluster to create")
	createCmd.Flags().StringVarP(&clusterConfigPath, "config", "c", "", "cluster config file")
	createCmd.Flags().StringVarP(&image, "image", "i", "quay.io/k0sproject/k0s:v1.33.3-k0s.0", "k0s image to use")
	createCmd.Flags().BoolVarP(&wait, "wait", "w", true, "wait for cluster to be ready")
	createCmd.Flags().StringVarP(&timeout, "timeout", "t", "60s", "timeout for cluster creation")
}

func runCreate(cmd *cobra.Command, args []string) error {
	clusterName := name
	var cc *k0daconfig.ClusterConfig
	if strings.TrimSpace(clusterConfigPath) != "" {
		var err error
		cc, err = k0daconfig.LoadClusterConfig(clusterConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load cluster config: %w", err)
		}
		if err := cc.Validate(); err != nil {
			return fmt.Errorf("invalid cluster config: %w", err)
		}
		// If config specifies image or version, derive image accordingly.
		if cc.Spec.K0s.Image != "" || cc.Spec.K0s.Version != "" {
			image = cc.Spec.K0s.EffectiveImage()
		}
	}

	fmt.Printf("Creating k0s cluster '%s'...\n", clusterName)

	// Detect container backend
	ctx := context.Background()
	r, err := runtime.Detect(ctx, runtime.DetectOptions{})
	if err != nil {
		return err
	}

	// Create cluster directory
	clusterDir := cc.ClusterDir(clusterName)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		return fmt.Errorf("failed to create cluster directory: %w", err)
	}

	// Write effective k0s config (defaults merged with inline user config) under the cluster directory and use it.
	var k0daConfigPath string
	if cc != nil {
		cfgDir := filepath.Join(clusterDir, "etc-k0s")
		if p, err := cc.WriteEffectiveK0sConfig(cfgDir); err != nil {
			return fmt.Errorf("failed to write effective k0s config: %w", err)
		} else if p != "" {
			k0daConfigPath = p
		}
	}

	// Create the cluster using backend
	if err := createK0sCluster(ctx, r, clusterName, image, wait, timeout, cc, k0daConfigPath); err != nil {
		return fmt.Errorf("failed to create k0s cluster: %w", err)
	}

	fmt.Printf("✅ Cluster '%s' created successfully!\n", clusterName)
	fmt.Printf("To use this cluster, run: kubectl config use-context k0da-%s\n", clusterName)

	return nil
}

func createK0sCluster(ctx context.Context, b runtime.Runtime, name, image string, wait bool, timeout string, cc *k0daconfig.ClusterConfig, k0sConfigHostPath string) error {
	containerName := name
	hostname := name
	volumeName := fmt.Sprintf("%s-var", name)

	fmt.Printf("Creating container '%s' with image '%s' using %s...\n", containerName, image, b.Name())

	// Build command args
	cmdArgs := []string{"k0s", "controller", "--no-taints", "--enable-dynamic-config"}
	if cc != nil && len(cc.Spec.Nodes) == 1 {
		cmdArgs = append(cmdArgs, "--single")
	} else {
		cmdArgs = append(cmdArgs, "--enable-worker")
	}
	if strings.TrimSpace(k0sConfigHostPath) != "" || (cc != nil && len(cc.Spec.K0s.Config) > 0) {
		cmdArgs = append(cmdArgs, "--config", "/etc/k0s/k0s.yaml")
	}
	// Global extra k0s args
	if cc != nil && len(cc.Spec.K0s.Args) > 0 {
		cmdArgs = append(cmdArgs, cc.Spec.K0s.Args...)
	}

	// Ensure manifests directory exists on host for k0s manifests and copy manifests into it
	hostK0daManifestsPath := cc.ManifestDir(name)
	if err := utils.CopyManifestsToDir(cc, hostK0daManifestsPath); err != nil {
		return fmt.Errorf("failed to stage manifests: %w", err)
	}

	// Build mounts
	mounts := runtime.Mounts{
		{Type: "volume", Source: fmt.Sprintf("%s", volumeName), Target: "/var"},
		{Type: "bind", Source: "/lib/modules", Target: "/lib/modules", Options: []string{"ro"}},
	}
	// Mount manifests directory into k0s manifests path
	mounts = append(mounts, runtime.Mount{Type: "bind", Source: hostK0daManifestsPath, Target: "/var/lib/k0s/manifests/k0da"})
	if strings.TrimSpace(k0sConfigHostPath) != "" || (cc != nil && len(cc.Spec.K0s.Config) > 0) {
		// Mount only the k0s.yaml file as read-only, leaving /etc/k0s writable
		mounts = append(mounts, runtime.Mount{Type: "bind", Source: k0sConfigHostPath, Target: "/etc/k0s/k0s.yaml", Options: []string{"ro"}})
	}

	// Docker socket mount is handled automatically by the Docker runtime when applicable

	// Node overrides/extensions
	var node *k0daconfig.NodeSpec
	if cc != nil {
		node = cc.PickPrimaryNode()
	}
	if node != nil {
		for _, m := range node.Mounts {
			mounts = append(mounts, runtime.Mount{Type: m.Type, Source: m.Source, Target: m.Target, Options: m.Options})
		}
	}

	// Ports to publish: honor node ports, ensure 6443/tcp exists at least once
	publish := []runtime.PortSpec{}
	if node != nil && len(node.Ports) > 0 {
		for _, p := range node.Ports {
			proto := strings.ToLower(p.Protocol)
			if proto == "" {
				proto = "tcp"
			}
			publish = append(publish, runtime.PortSpec{ContainerPort: p.ContainerPort, Protocol: proto, HostIP: p.HostIP, HostPort: p.HostPort})
		}
	}
	// Ensure API port mapping exists
	hasAPI := false
	for _, ps := range publish {
		if ps.ContainerPort == 6443 && (ps.Protocol == "" || strings.ToLower(ps.Protocol) == "tcp") {
			hasAPI = true
			break
		}
	}
	if !hasAPI {
		publish = append(publish, runtime.PortSpec{ContainerPort: 6443, Protocol: "tcp"})
	}
	// Ensure API port is explicitly mapped to a fixed host port
	// Locate API mapping entry
	var apiPortIndex = -1
	for i := range publish {
		if publish[i].ContainerPort == 6443 && (publish[i].Protocol == "" || strings.ToLower(publish[i].Protocol) == "tcp") {
			apiPortIndex = i
			break
		}
	}
	if apiPortIndex != -1 {
		if publish[apiPortIndex].HostPort == 0 {
			hostIP := publish[apiPortIndex].HostIP
			if p, err := utils.AllocateHostPort(hostIP); err == nil && p > 0 {
				publish[apiPortIndex].HostPort = p
			}
		}
	}

	// Env vars
	var env runtime.EnvVars
	if node != nil && len(node.Env) > 0 {
		for k, v := range node.Env {
			env = append(env, runtime.EnvVar{Name: k, Value: v})
		}
	}

	// Labels
	labels := map[string]string{"k0da.cluster": "true", "k0da.cluster.name": name, "k0da.cluster.type": "k0s"}
	if node != nil && len(node.Labels) > 0 {
		for k, v := range node.Labels {
			labels[k] = v
		}
	}

	// Node-specific k0s args at the end
	if node != nil && len(node.Args) > 0 {
		cmdArgs = append(cmdArgs, node.Args...)
	}

	// Effective image with node override
	effectiveImage := image
	if node != nil && strings.TrimSpace(node.Image) != "" {
		effectiveImage = node.Image
	}

	// Ensure network exists and attach container to it (kind-like shared network)
	networkName := k0daconfig.DefaultNetwork
	if cc != nil {
		networkName = cc.Spec.Options.Network
	}
	if err := b.EnsureNetwork(ctx, networkName); err != nil {
		return fmt.Errorf("failed to ensure network: %w", err)
	}

	// Tmpfs mounts: always mount /run and /var/run
	tmpfs := map[string]string{"/run": "", "/var/run": ""}

	_, err := b.RunContainer(ctx, runtime.RunContainerOptions{
		Name:        containerName,
		Hostname:    hostname,
		Image:       effectiveImage,
		Args:        cmdArgs,
		Env:         env,
		Labels:      labels,
		Mounts:      mounts,
		Tmpfs:       tmpfs,
		SecurityOpt: []string{"seccomp=unconfined", "apparmor=unconfined", "label=disable"},
		Privileged:  true,
		//AutoRemove:  true,
		Publish: publish,
		Network: networkName,
	})
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	// No file persistence: explicit HostPort ensures Docker/Podman keep mapping across daemon restarts

	fmt.Printf("✅ Container created successfully\n")

	if wait {
		fmt.Println("Waiting for cluster to be ready...")
		if err := utils.WaitForK0sReady(ctx, b, containerName, timeout); err != nil {
			return fmt.Errorf("cluster failed to become ready: %w", err)
		}
		fmt.Println("✅ Cluster is ready!")

		// Add cluster to unified kubeconfig
		if err := utils.AddClusterToKubeconfig(ctx, b, name, containerName); err != nil {
			return fmt.Errorf("failed to add cluster to kubeconfig: %w", err)
		}
	}

	return nil
}

// copyManifestsToDir copies provided manifest file paths into destination directory.
// Paths are resolved relative to baseDir when not absolute. Files are written
// into destDir with a numeric prefix to preserve ordering when provided.
func copyManifestsToDir(paths []string, baseDir string, destDir string) error {
	for i, mp := range paths {
		p := strings.TrimSpace(mp)
		if p == "" {
			continue
		}
		abs := p
		if !filepath.IsAbs(p) && strings.TrimSpace(baseDir) != "" {
			abs = filepath.Join(baseDir, p)
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("failed to read manifest %q: %w", p, err)
		}
		// Prefix with index to keep deterministic order
		dst := filepath.Join(destDir, fmt.Sprintf("%03d_%s", i, filepath.Base(abs)))
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("failed to write manifest to %q: %w", dst, err)
		}
	}
	return nil
}
