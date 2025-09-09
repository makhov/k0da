package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	k0daconfig "github.com/makhov/k0da/internal/config"
	"github.com/makhov/k0da/internal/runtime"
	"github.com/makhov/k0da/internal/utils"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:     "delete [cluster-name]",
	Aliases: []string{"rm", "del", "remove"},
	Short:   "Delete a k0s cluster",
	Long: `Delete a k0s cluster with the specified name.
This command will stop and remove the container associated with the cluster.
The cluster name can be provided as an argument or via the --name flag.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDelete,
}

var (
	deleteName string
	force      bool
)

func init() {
	rootCmd.AddCommand(deleteCmd)

	// Here you will define your flags and configuration settings.
	deleteCmd.Flags().StringVarP(&deleteName, "name", "n", DefaultClusterName, "name of the cluster to delete")
	deleteCmd.Flags().BoolVarP(&force, "force", "f", false, "force delete without confirmation")
}

func runDelete(cmd *cobra.Command, args []string) error {
	clusterName := deleteName
	if len(args) > 0 {
		clusterName = args[0]
	}

	if clusterName == "" {
		return fmt.Errorf("cluster name is required. Use --name flag or provide as argument")
	}

	ctx := context.Background()
	r, err := runtime.Detect(ctx, runtime.DetectOptions{})
	if err != nil {
		return err
	}

	// Find all containers for this cluster and delete them
	list, err := r.ListContainersByLabel(ctx, map[string]string{k0daconfig.LabelClusterName: clusterName}, true)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return fmt.Errorf("cluster '%s' not found", clusterName)
	}
	// Stop running containers first
	for _, c := range list {
		running, err := r.ContainerIsRunning(ctx, c.Name)
		if err == nil && running {
			fmt.Printf("Stopping node '%s'...\n", c.Name)
			_ = r.StopContainer(ctx, c.Name)
		}
	}
	for _, c := range list {
		fmt.Printf("Deleting node '%s'...\n", c.Name)
		if err := r.RemoveContainer(ctx, c.Name); err != nil {
			fmt.Printf("Warning: failed to remove container %s: %v\n", c.Name, err)
		}
		// Remove its volume
		volName := fmt.Sprintf("%s-var", c.Name)
		if exists, _ := r.VolumeExists(ctx, volName); exists {
			fmt.Printf("Removing volume '%s'...\n", volName)
			if err := r.RemoveVolume(ctx, volName); err != nil {
				fmt.Printf("Warning: failed to remove volume '%s': %v\n", volName, err)
			}
		}
	}

	// Remove cluster from unified kubeconfig
	if err := utils.RemoveClusterFromKubeconfig(clusterName); err != nil {
		fmt.Printf("Warning: failed to remove cluster from kubeconfig: %v\n", err)
	}

	// Remove cluster working directory under $HOME/.k0da/clusters/<name>
	if home, err := os.UserHomeDir(); err == nil {
		dir := filepath.Join(home, ".k0da", "clusters", clusterName)
		if err := os.RemoveAll(dir); err != nil {
			fmt.Printf("Warning: failed to remove cluster directory %s: %v\n", dir, err)
		}
	}

	fmt.Printf("✅ Cluster '%s' deleted successfully!\n", clusterName)
	return nil
}
