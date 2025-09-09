package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	createCmd.Flags().StringVarP(&image, "image", "i", k0daconfig.DefaultK0sImageRepo+":"+k0daconfig.DefaultK0sVersion, "k0s image to use")
	createCmd.Flags().BoolVarP(&wait, "wait", "w", true, "wait for cluster to be ready")
	createCmd.Flags().StringVarP(&timeout, "timeout", "t", "60s", "timeout for cluster creation")
}

func runCreate(cmd *cobra.Command, args []string) error {
	clusterName := name

	// Load cluster config (always returns a valid config)
	cc, err := k0daconfig.LoadClusterConfig(strings.TrimSpace(clusterConfigPath))
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}

	// Determine final image with precedence: config > user-flag override > fetched stable > default
	var finalImage string
	if cc.Spec.K0s.Image != "" || cc.Spec.K0s.Version != "" {
		finalImage = cc.Spec.K0s.EffectiveImage()
	} else {
		client := &http.Client{Timeout: 3 * time.Second}
		if stable, err := k0daconfig.FetchStableK0sVersion(client); err == nil && strings.TrimSpace(stable) != "" {
			finalImage = k0daconfig.DefaultK0sImageRepo + ":" + k0daconfig.NormalizeVersionTag(stable)
		} else {
			finalImage = k0daconfig.DefaultK0sImageRepo + ":" + k0daconfig.DefaultK0sVersion
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

	// Always write effective k0s config (defaults merged with inline user config) for consistency
	cfgDir := filepath.Join(clusterDir, "etc-k0s")
	p, err := cc.WriteEffectiveK0sConfig(cfgDir)
	if err != nil {
		return fmt.Errorf("failed to write effective k0s config: %w", err)
	}
	var k0daConfigPath string
	if p != "" {
		k0daConfigPath = p
	}

	// Create the primary node/container using backend
	if err := createK0sCluster(ctx, r, clusterName, finalImage, wait, timeout, cc, k0daConfigPath); err != nil {
		return fmt.Errorf("failed to create k0s cluster: %w", err)
	}

	// If multinode defined, join additional nodes to the primary
	if cc != nil && len(cc.Spec.Nodes) > 1 {
		if err := joinAdditionalNodes(ctx, r, clusterName, image, wait, timeout, cc); err != nil {
			return fmt.Errorf("failed to join additional nodes: %w", err)
		}
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
	if len(cc.Spec.Nodes) == 1 {
		cmdArgs = append(cmdArgs, "--single")
	} else {
		cmdArgs = append(cmdArgs, "--enable-worker")
	}
	if strings.TrimSpace(k0sConfigHostPath) != "" {
		cmdArgs = append(cmdArgs, "--config", "/etc/k0s/k0s.yaml")
	}
	// Global extra k0s args
	if len(cc.Spec.K0s.Args) > 0 {
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
	if strings.TrimSpace(k0sConfigHostPath) != "" {
		// Mount only the k0s.yaml file as read-only, leaving /etc/k0s writable
		mounts = append(mounts, runtime.Mount{Type: "bind", Source: k0sConfigHostPath, Target: "/etc/k0s/k0s.yaml", Options: []string{"ro"}})
	}

	// Node overrides/extensions
	node := cc.PickPrimaryNode()
	if node != nil {
		for _, m := range node.Mounts {
			mounts = append(mounts, runtime.Mount{Type: m.Type, Source: m.Source, Target: m.Target, Options: m.Options})
		}
	}

	// Ports, Env, Labels
	publish := buildPublishPortsFromNode(node)
	publish = ensureAPIExposed(publish)
	publish = ensureAPIPortBound(publish)
	env := buildEnvFromNode(node)
	labels := buildLabelsForNode(name, name, "controller", node)

	// Effective image with node override
	effectiveImage := image
	if node != nil && strings.TrimSpace(node.Image) != "" {
		effectiveImage = node.Image
	}

	// Ensure network exists and attach container to it (kind-like shared network)
	networkName := cc.Spec.Options.Network
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
		Publish:     publish,
		Network:     networkName,
	})
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

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

// joinAdditionalNodes creates tokens on the primary node and starts additional nodes defined in the config.
func joinAdditionalNodes(ctx context.Context, b runtime.Runtime, clusterName, image string, wait bool, timeout string, cc *k0daconfig.ClusterConfig) error {
	primary := clusterName
	clusterDir := filepath.Join(os.Getenv("HOME"), ".k0da", "clusters", clusterName)
	tokensDir := filepath.Join(clusterDir, "tokens")
	if err := os.MkdirAll(tokensDir, 0755); err != nil {
		return fmt.Errorf("create tokens dir: %w", err)
	}

	networkName := k0daconfig.DefaultNetwork
	if cc != nil {
		networkName = cc.Spec.Options.Network
	}
	if err := b.EnsureNetwork(ctx, networkName); err != nil {
		return fmt.Errorf("failed to ensure network: %w", err)
	}

	primaryNode := cc.PickPrimaryNode()
	idx := 0
	for i := range cc.Spec.Nodes {
		n := &cc.Spec.Nodes[i]
		if primaryNode != nil && &cc.Spec.Nodes[i] == primaryNode {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(n.Role))
		if role == "" {
			role = "worker"
		}
		tokenOut, exit, err := b.ExecInContainer(ctx, primary, []string{"k0s", "token", "create", "--role=" + role})
		if err != nil || exit != 0 {
			return fmt.Errorf("failed to create %s token on primary: %v", role, err)
		}
		token := strings.TrimSpace(tokenOut)
		nodeName := strings.TrimSpace(n.Name)
		if nodeName == "" {
			nodeName = fmt.Sprintf("%s-%s-%d", clusterName, role, idx)
			idx++
		}
		hostTokenPath := filepath.Join(tokensDir, nodeName+".token")
		if err := os.WriteFile(hostTokenPath, []byte(token+"\n"), 0600); err != nil {
			return fmt.Errorf("write token file: %v", err)
		}

		var cmdArgs []string
		switch role {
		case "controller":
			cmdArgs = []string{"k0s", "controller", "--token-file", "/etc/k0s/join.token"}
		default:
			cmdArgs = []string{"k0s", "worker", "--token-file", "/etc/k0s/join.token"}
		}
		if len(n.Args) > 0 {
			cmdArgs = append(cmdArgs, n.Args...)
		}

		volumeName := fmt.Sprintf("%s-var", nodeName)
		mounts := runtime.Mounts{
			{Type: "volume", Source: volumeName, Target: "/var"},
			{Type: "bind", Source: "/lib/modules", Target: "/lib/modules", Options: []string{"ro"}},
			{Type: "bind", Source: hostTokenPath, Target: "/etc/k0s/join.token", Options: []string{"ro"}},
		}

		publish := buildPublishPortsFromNode(n)
		// Env, Labels
		env := buildEnvFromNode(n)
		labels := buildLabelsForNode(clusterName, nodeName, role, n)

		effectiveImage := image
		if strings.TrimSpace(n.Image) != "" {
			effectiveImage = n.Image
		}

		_, err = b.RunContainer(ctx, runtime.RunContainerOptions{
			Name:        nodeName,
			Hostname:    nodeName,
			Image:       effectiveImage,
			Args:        cmdArgs,
			Env:         env,
			Labels:      labels,
			Mounts:      mounts,
			Tmpfs:       map[string]string{"/run": "", "/var/run": ""},
			SecurityOpt: []string{"seccomp=unconfined", "apparmor=unconfined", "label=disable"},
			Privileged:  true,
			Publish:     publish,
			Network:     networkName,
		})
		if err != nil {
			return fmt.Errorf("failed to start node %s: %w", nodeName, err)
		}
		if wait {
			// Only wait for controller nodes; workers don't expose the same status
			if role == "controller" {
				if err := utils.WaitForK0sReady(ctx, b, nodeName, timeout); err != nil {
					return fmt.Errorf("node %s failed to become ready: %w", nodeName, err)
				}
			}
		}
	}
	return nil
}

// Helpers
func buildPublishPortsFromNode(node *k0daconfig.NodeSpec) []runtime.PortSpec {
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
	return publish
}

func ensureAPIExposed(publish []runtime.PortSpec) []runtime.PortSpec {
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
	return publish
}

func ensureAPIPortBound(publish []runtime.PortSpec) []runtime.PortSpec {
	for i := range publish {
		if publish[i].ContainerPort == 6443 && (publish[i].Protocol == "" || strings.ToLower(publish[i].Protocol) == "tcp") {
			if publish[i].HostPort == 0 {
				hostIP := publish[i].HostIP
				if p, err := utils.AllocateHostPort(hostIP); err == nil && p > 0 {
					publish[i].HostPort = p
				}
			}
			break
		}
	}
	return publish
}

func buildEnvFromNode(node *k0daconfig.NodeSpec) runtime.EnvVars {
	var env runtime.EnvVars
	if node != nil && len(node.Env) > 0 {
		for k, v := range node.Env {
			env = append(env, runtime.EnvVar{Name: k, Value: v})
		}
	}
	return env
}

func buildLabelsForNode(clusterName, nodeName, role string, node *k0daconfig.NodeSpec) map[string]string {
	labels := map[string]string{k0daconfig.LabelCluster: "true", k0daconfig.LabelClusterName: clusterName, k0daconfig.LabelClusterType: "k0s", k0daconfig.LabelNodeName: nodeName, k0daconfig.LabelNodeRole: role}
	if node != nil && len(node.Labels) > 0 {
		for k, v := range node.Labels {
			labels[k] = v
		}
	}
	return labels
}
