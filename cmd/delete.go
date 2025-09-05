package cmd

import (
	"context"
	"fmt"
	"github.com/makhov/k0da/internal/runtime"
	"github.com/makhov/k0da/internal/utils"
	"github.com/spf13/cobra"
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

	containerName := clusterName
	volumeName := fmt.Sprintf("%s-var", clusterName)

	// Check if container exists or if volume exists (for cleanup of --rm containers)
	exists, err := r.ContainerExists(ctx, containerName)
	if err != nil {
		return err
	}
	volExists, err := r.VolumeExists(ctx, volumeName)
	if err != nil {
		return err
	}

	if !exists && !volExists {
		return fmt.Errorf("cluster '%s' not found (no container or volume found)", clusterName)
	}

	// Handle container if it exists
	if exists {
		// Check if container is running
		isRunning, err := r.ContainerIsRunning(ctx, containerName)
		if err != nil {
			return err
		}
		if isRunning {
			fmt.Printf("Stopping cluster '%s'...\n", clusterName)
			if err := r.StopContainer(ctx, containerName); err != nil {
				return fmt.Errorf("failed to stop container: %w", err)
			}
		}

		// Remove container
		fmt.Printf("Deleting cluster '%s'...\n", clusterName)
		if err := r.RemoveContainer(ctx, containerName); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	} else if volExists {
		fmt.Printf("Container for cluster '%s' was already removed, cleaning up volume...\n", clusterName)
	}

	// Remove associated volume
	if volExists {
		fmt.Printf("Removing volume '%s'...\n", volumeName)
		if err := r.RemoveVolume(ctx, volumeName); err != nil {
			fmt.Printf("Warning: failed to remove volume '%s': %v\n", volumeName, err)
		}
	}

	// Remove cluster from unified kubeconfig
	if err := utils.RemoveClusterFromKubeconfig(clusterName); err != nil {
		fmt.Printf("Warning: failed to remove cluster from kubeconfig: %v\n", err)
	}

	fmt.Printf("✅ Cluster '%s' deleted successfully!\n", clusterName)
	return nil
}
