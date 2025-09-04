package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	Short: "Load a tar archive or OCI layout dir into cluster's containerd",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := args[0]
		return runLoadArchive(loadName, src)
	},
}

var loadImageCmd = &cobra.Command{
	Use:   "image [image-ref]",
	Short: "Pull and load a container image into cluster's containerd",
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
	b, err := runtime.Detect(ctx, runtime.DetectOptions{})
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
	// Copy to container /tmp
	inContainer := "/tmp/" + filepath.Base(abs)
	if err := b.CopyToContainer(ctx, clusterName, abs, inContainer); err != nil {
		return err
	}
	// Import via k0s ctr
	out, code, _ := b.ExecInContainer(ctx, clusterName, []string{"k0s", "ctr", "-n", "k8s.io", "images", "import", inContainer})
	if code != 0 {
		return fmt.Errorf("import failed: %s", out)
	}
	fmt.Println("✅ archive loaded")
	return nil
}

func runLoadImage(clusterName, imageRef string) error {
	ctx := context.Background()
	b, err := runtime.Detect(ctx, runtime.DetectOptions{})
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
	defer os.RemoveAll(tmpDir)
	tarPath := filepath.Join(tmpDir, "image.tar")

	if err := b.SaveImageToTar(ctx, imageRef, tarPath); err != nil {
		return fmt.Errorf("failed to save local image: %w", err)
	}
	inContainer := "/tmp/" + filepath.Base(tarPath)
	if err := b.CopyToContainer(ctx, name, tarPath, inContainer); err != nil {
		return fmt.Errorf("failed to copy image tar: %w", err)
	}
	out, code, _ := b.ExecInContainer(ctx, name, []string{"k0s", "ctr", "-n", "k8s.io", "images", "import", inContainer})
	if code != 0 {
		return fmt.Errorf("import failed: %s", out)
	}
	fmt.Println("✅ image loaded from local runtime")
	return nil
}
