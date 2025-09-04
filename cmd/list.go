package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/makhov/k0da/internal/runtime"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all k0da clusters",
	Long: `List all k0da clusters that are currently running or stopped.
This command shows clusters managed by k0da using container labels.`,
	RunE: runList,
}

var (
	all     bool
	verbose bool
)

func init() {
	rootCmd.AddCommand(listCmd)

	// Here you will define your flags and configuration settings.
	listCmd.Flags().BoolVarP(&all, "all", "a", false, "show all clusters including stopped ones")
	listCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show detailed information")
}

func runList(cmd *cobra.Command, args []string) error {
	clusters, err := getK0daClusters(all)
	if err != nil {
		return fmt.Errorf("failed to get clusters: %w", err)
	}

	if len(clusters) == 0 {
		fmt.Println("No k0da clusters found.")
		return nil
	}

	if verbose {
		printVerboseList(clusters)
	} else {
		printSimpleList(clusters)
	}

	return nil
}

type ClusterInfo struct {
	Name        string `json:"name"`
	ContainerID string `json:"container_id"`
	Image       string `json:"image"`
	Status      string `json:"status"`
	Ports       string `json:"ports"`
	Created     string `json:"created"`
}

func getK0daClusters(includeStopped bool) ([]ClusterInfo, error) {
	ctx := context.Background()
	b, err := runtime.Detect(ctx, runtime.DetectOptions{})
	if err != nil {
		return nil, err
	}

	selector := map[string]string{"k0da.cluster": "true"}
	list, err := b.ListContainersByLabel(ctx, selector, includeStopped)
	if err != nil {
		return nil, err
	}

	clusters := make([]ClusterInfo, 0, len(list))
	for _, c := range list {
		name := c.Name
		if v, ok := c.Labels["k0da.cluster.name"]; ok && strings.TrimSpace(v) != "" {
			name = v
		}
		id := c.ID
		if len(id) > 12 {
			id = id[:12]
		}
		clusters = append(clusters, ClusterInfo{
			Name:        name,
			ContainerID: id,
			Image:       c.Image,
			Status:      c.Status,
			Ports:       c.Ports,
			Created:     fmt.Sprintf("%d", c.Created),
		})
	}

	return clusters, nil
}

func printSimpleList(clusters []ClusterInfo) {
	fmt.Printf("Found %d k0da cluster(s):\n\n", len(clusters))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tPORTS\tIMAGE")
	fmt.Fprintln(w, "----\t------\t-----\t-----")

	for _, cluster := range clusters {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			cluster.Name,
			cluster.Status,
			cluster.Ports,
			cluster.Image)
	}

	w.Flush()
}

func printVerboseList(clusters []ClusterInfo) {
	fmt.Printf("Found %d k0da cluster(s):\n\n", len(clusters))

	for i, cluster := range clusters {
		fmt.Printf("Cluster %d:\n", i+1)
		fmt.Printf("  Name:        %s\n", cluster.Name)
		fmt.Printf("  Container:   %s\n", cluster.ContainerID)
		fmt.Printf("  Image:       %s\n", cluster.Image)
		fmt.Printf("  Status:      %s\n", cluster.Status)
		fmt.Printf("  Ports:       %s\n", cluster.Ports)
		fmt.Printf("  Created:     %s\n", cluster.Created)
		fmt.Println()
	}
}
