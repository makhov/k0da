package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/makhov/k0da/internal/utils"
	"github.com/spf13/cobra"
)

var (
	kubeconfigClusterName string
)

// kubeconfigCmd represents the kubeconfig command
var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Print kubeconfig for a specific cluster",
	Long: `Print the kubeconfig for a specific k0da cluster.
This command extracts the kubeconfig for the specified cluster from the unified kubeconfig
and prints it to stdout, making it easy to use with kubectl or other tools.`,
	RunE: runKubeconfig,
}

func init() {
	rootCmd.AddCommand(kubeconfigCmd)
	kubeconfigCmd.Flags().StringVarP(&kubeconfigClusterName, "name", "n", DefaultClusterName, "name of the cluster (required)")
}

func runKubeconfig(cmd *cobra.Command, args []string) error {
	unifiedKubeconfigPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")

	// Check if unified kubeconfig exists
	if _, err := os.Stat(unifiedKubeconfigPath); os.IsNotExist(err) {
		return fmt.Errorf("no unified kubeconfig found at %s. Create a cluster first", unifiedKubeconfigPath)
	}

	// Load the unified kubeconfig
	kubeconfig, err := utils.LoadKubeconfig(unifiedKubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load unified kubeconfig: %w", err)
	}
	// Find the cluster context
	clusterContext := "k0da-" + kubeconfigClusterName
	var foundContext *utils.NamedContext
	for _, context := range kubeconfig.Contexts {
		if context.Name == clusterContext {
			foundContext = &context
			break
		}
	}

	if foundContext == nil {
		return fmt.Errorf("cluster '%s' not found. Available clusters: %v", kubeconfigClusterName, getClusterNames(kubeconfig.Contexts))
	}

	// Create a new kubeconfig with only the specified cluster
	clusterKubeconfig := &utils.Kubeconfig{
		APIVersion:     kubeconfig.APIVersion,
		Kind:           kubeconfig.Kind,
		CurrentContext: clusterContext,
		Clusters:       []utils.NamedCluster{},
		Contexts:       []utils.NamedContext{},
		Users:          []utils.NamedUser{},
	}

	// Find and add the cluster
	for _, cluster := range kubeconfig.Clusters {
		if cluster.Name == foundContext.Context.Cluster {
			clusterKubeconfig.Clusters = append(clusterKubeconfig.Clusters, cluster)
			break
		}
	}

	// Add the context
	clusterKubeconfig.Contexts = append(clusterKubeconfig.Contexts, *foundContext)

	// Find and add the user
	for _, user := range kubeconfig.Users {
		if user.Name == foundContext.Context.User {
			clusterKubeconfig.Users = append(clusterKubeconfig.Users, user)
			break
		}
	}

	// Marshal and print the kubeconfig
	data, err := utils.MarshalKubeconfig(clusterKubeconfig)
	if err != nil {
		return fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

func getClusterNames(contexts []utils.NamedContext) []string {
	names := make([]string, 0, len(contexts))
	for _, context := range contexts {
		if len(context.Name) > 5 && context.Name[:5] == "k0da-" {
			names = append(names, context.Name[5:])
		}
	}
	return names
}
