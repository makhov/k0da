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

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [cluster-name]",
	Short: "Update an existing k0s cluster",
	Long: `Update an existing k0s cluster.
This command (re)writes the effective k0s config from the provided cluster config,
updates staged manifests. k0s will auto-apply manifest changes without restart.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUpdate,
}

var (
	updateName       string
	updateClusterCfg string
	updateImage      string
	updateTimeout    string
)

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringVarP(&updateName, "name", "n", DefaultClusterName, "name of the cluster to update")
	updateCmd.Flags().StringVarP(&updateClusterCfg, "config", "c", "", "cluster config file")
	updateCmd.Flags().StringVarP(&updateImage, "image", "i", "quay.io/k0sproject/k0s:v1.33.3-k0s.0", "k0s image to use (overrides config)")
	updateCmd.Flags().StringVarP(&updateTimeout, "timeout", "t", "60s", "timeout for readiness wait")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	clusterName := updateName
	if len(args) > 0 {
		clusterName = args[0]
	}
	if strings.TrimSpace(clusterName) == "" {
		return fmt.Errorf("cluster name is required")
	}

	// Load cluster config if provided
	var cc *k0daconfig.ClusterConfig
	if strings.TrimSpace(updateClusterCfg) != "" {
		var err error
		cc, err = k0daconfig.LoadClusterConfig(updateClusterCfg)
		if err != nil {
			return fmt.Errorf("failed to load cluster config: %w", err)
		}
		if err := cc.Validate(); err != nil {
			return fmt.Errorf("invalid cluster config: %w", err)
		}
		if updateImage == "" || updateImage == "quay.io/k0sproject/k0s:v1.33.3-k0s.0" {
			if cc.Spec.K0s.Image != "" || cc.Spec.K0s.Version != "" {
				updateImage = cc.Spec.K0s.EffectiveImage()
			}
		}
	}

	fmt.Printf("Updating k0s cluster '%s'...\n", clusterName)

	// Detect container backend
	ctx := context.Background()
	r, err := runtime.Detect(ctx, runtime.DetectOptions{})
	if err != nil {
		return err
	}

	// Ensure cluster work dir exists
	clusterDir := cc.ClusterDir(clusterName)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		return fmt.Errorf("failed to create cluster directory: %w", err)
	}

	if err := utils.CopyManifestsToDir(cc, cc.ManifestDir(clusterName)); err != nil {
		return fmt.Errorf("failed to stage manifests: %w", err)
	}

	// Apply dynamic k0s config in-cluster if provided
	if cc != nil {
		etcDir := filepath.Join(clusterDir, "etc-k0s")
		p, err := cc.WriteEffectiveK0sConfig(etcDir)
		if err != nil {
			return fmt.Errorf("failed to write effective k0s config: %w", err)
		}
		_ = p // mounted at /etc/k0s/k0s.yaml inside the container
		if out, exit, err := r.ExecInContainer(ctx, clusterName, []string{"k0s", "kc", "apply", "-f", "/etc/k0s/k0s.yaml"}); err != nil || exit != 0 {
			return fmt.Errorf("failed to apply dynamic config via k0s: %v, out: %s", err, out)
		}
	}

	fmt.Printf("✅ Cluster '%s' updated successfully!\n", clusterName)
	return nil
}
