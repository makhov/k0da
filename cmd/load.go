package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	k0daconfig "github.com/makhov/k0da/internal/config"
	"github.com/makhov/k0da/internal/runtime"
	"github.com/spf13/cobra"
)

var loadCmd = &cobra.Command{
	Use:   "load",
	Short: "Load images into the k0s cluster",
}

var (
	loadName string
)

var loadArchiveCmd = &cobra.Command{
	Use:   "archive [tar-archive-or-dir]",
	Short: "Load a tar archive or OCI layout dir into the containerd of each of the cluster's nodes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := args[0]
		return runLoadArchive(loadName, src)
	},
}

var loadImageCmd = &cobra.Command{
	Use:   "image [image-ref]",
	Short: "Load a container image from the local runtime into the containerd of each of the cluster's nodes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		imageRef := args[0]
		return runLoadImage(loadName, imageRef)
	},
}

func init() {
	rootCmd.AddCommand(loadCmd)
	loadCmd.AddCommand(loadArchiveCmd)
	loadCmd.AddCommand(loadImageCmd)

	// --name flag with default from constant
	loadCmd.PersistentFlags().StringVarP(&loadName, "name", "n", DefaultClusterName, "name of the cluster")
}

func runLoadArchive(clusterName, src string) error {
	ctx := context.Background()
	b, err := runtime.Detect(ctx)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("source not found: %s", abs)
	}
	if err := importToNodes(ctx, b, clusterName, abs); err != nil {
		return err
	}
	fmt.Println("✅ archive loaded")
	return nil
}

func runLoadImage(clusterName, imageRef string) error {
	ctx := context.Background()
	b, err := runtime.Detect(ctx)
	if err != nil {
		return err
	}
	name := clusterName
	// If imageRef looks like a local tar file, delegate to archive path
	if strings.HasSuffix(imageRef, ".tar") || strings.HasSuffix(imageRef, ".tar.gz") || strings.HasSuffix(imageRef, ".tgz") {
		return runLoadArchive(clusterName, imageRef)
	}
	// Save local runtime image to a temporary tar and import it
	tmpDir, err := os.MkdirTemp("", "k0da-img-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	tarPath := filepath.Join(tmpDir, "image.tar")

	if err := b.SaveImageToTar(ctx, imageRef, tarPath); err != nil {
		return fmt.Errorf("failed to save local image: %w", err)
	}
	if err := importToNodes(ctx, b, name, tarPath); err != nil {
		return err
	}
	fmt.Println("✅ image loaded from local runtime")
	return nil
}

// importToNodes imports the image archive at src into the containerd of
// every running node of the cluster: a pod may be scheduled on any of
// them.
func importToNodes(ctx context.Context, b runtime.Runtime, clusterName, src string) error {
	nodes, err := b.ListContainersByLabel(ctx, map[string]string{k0daconfig.LabelClusterName: clusterName}, false)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("cluster '%s' has no running nodes", clusterName)
	}
	inContainer := "/tmp/" + filepath.Base(src)
	for _, n := range nodes {
		if err := b.CopyToContainer(ctx, n.Name, src, inContainer); err != nil {
			return fmt.Errorf("node %s: copy: %w", n.Name, err)
		}
		out, code, err := b.ExecInContainer(ctx, n.Name, []string{"k0s", "ctr", "-n", "k8s.io", "images", "import", inContainer})
		_, _, _ = b.ExecInContainer(ctx, n.Name, []string{"rm", "-f", inContainer})
		if err != nil {
			return fmt.Errorf("node %s: import failed: %w", n.Name, err)
		}
		if code != 0 {
			return fmt.Errorf("node %s: import failed: %s", n.Name, out)
		}
		fmt.Printf("loaded into node '%s'\n", n.Name)
	}
	return nil
}
