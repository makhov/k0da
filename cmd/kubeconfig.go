package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/makhov/k0da/internal/runtime"
	"github.com/makhov/k0da/internal/utils"
	"github.com/spf13/cobra"
)

var (
	kubeconfigClusterName string
)

// kubeconfigCmd represents the kubeconfig command
var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig [cluster-name]",
	Short: "Print kubeconfig for a specific cluster",
	Long: `Print the kubeconfig for a specific k0da cluster.
The kubeconfig comes from the cluster's controller, so it works wherever
k0da create wrote its copy (the first file of $KUBECONFIG, or ~/.kube/config).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runKubeconfig,
}

func init() {
	rootCmd.AddCommand(kubeconfigCmd)
	kubeconfigCmd.Flags().StringVarP(&kubeconfigClusterName, "name", "n", DefaultClusterName, "name of the cluster")
}

func runKubeconfig(cmd *cobra.Command, args []string) error {
	name := kubeconfigClusterName
	if len(args) > 0 {
		name = args[0]
	}
	ctx := context.Background()
	b, err := runtime.Detect(ctx)
	if err != nil {
		return err
	}
	return printKubeconfig(ctx, b, name, os.Stdout)
}

// printKubeconfig writes the admin kubeconfig of the named cluster to w.
// The cluster's controller is the container named after the cluster.
func printKubeconfig(ctx context.Context, b runtime.Runtime, name string, w io.Writer) error {
	running, err := b.ContainerIsRunning(ctx, name)
	if err != nil || !running {
		return fmt.Errorf("cluster '%s' not found or not running (see k0da list)", name)
	}
	kc, err := utils.ClusterKubeconfig(ctx, b, name, name)
	if err != nil {
		return fmt.Errorf("cluster '%s': %w", name, err)
	}
	data, err := utils.MarshalKubeconfig(kc)
	if err != nil {
		return fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}
	_, err = w.Write(data)
	return err
}
