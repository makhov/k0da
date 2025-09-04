package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/makhov/k0da/internal/utils"
	"github.com/spf13/cobra"
)

var (
	contextName string
)

// contextCmd represents the context command
var contextCmd = &cobra.Command{
	Use:   "context [context-name]",
	Short: "Switch to a different k0da cluster context",
	Long: `Switch to a different k0da cluster context in the unified kubeconfig.
This command allows you to switch between different k0da clusters without
specifying the kubeconfig file each time.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runContext,
}

func init() {
	rootCmd.AddCommand(contextCmd)
	contextCmd.Flags().StringVarP(&contextName, "name", "n", "", "name of the context to switch to")
}

func runContext(cmd *cobra.Command, args []string) error {
	unifiedKubeconfigPath := filepath.Join(os.Getenv("HOME"), ".k0da", "clusters", "kubeconfig")

	// Check if unified kubeconfig exists
	if _, err := os.Stat(unifiedKubeconfigPath); os.IsNotExist(err) {
		return fmt.Errorf("no unified kubeconfig found at %s. Create a cluster first", unifiedKubeconfigPath)
	}

	// Load the unified kubeconfig
	kubeconfig, err := utils.LoadKubeconfig(unifiedKubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to load unified kubeconfig: %w", err)
	}

	// If no arguments provided, show current context and available contexts
	if len(args) == 0 && contextName == "" {
		fmt.Printf("Current context: %s\n", kubeconfig.CurrentContext)
		fmt.Println("\nAvailable contexts:")
		for _, context := range kubeconfig.Contexts {
			marker := " "
			if context.Name == kubeconfig.CurrentContext {
				marker = "*"
			}
			fmt.Printf("  %s %s\n", marker, context.Name)
		}
		return nil
	}

	// Get the target context name
	targetContext := contextName
	if len(args) > 0 {
		targetContext = args[0]
	}

	// Validate that the context exists
	contextExists := false
	for _, context := range kubeconfig.Contexts {
		if context.Name == targetContext {
			contextExists = true
			break
		}
	}

	if !contextExists {
		return fmt.Errorf("context '%s' not found. Available contexts: %v", targetContext, getContextNames(kubeconfig.Contexts))
	}

	// Switch to the context
	kubeconfig.CurrentContext = targetContext

	// Save the updated kubeconfig
	if err := utils.SaveKubeconfig(kubeconfig, unifiedKubeconfigPath); err != nil {
		return fmt.Errorf("failed to save kubeconfig: %w", err)
	}

	fmt.Printf("✅ Switched to context '%s'\n", targetContext)
	return nil
}

func getContextNames(contexts []utils.NamedContext) []string {
	names := make([]string, len(contexts))
	for i, context := range contexts {
		names[i] = context.Name
	}
	return names
}
