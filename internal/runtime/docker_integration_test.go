package runtime

import (
	"archive/tar"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDockerAPI exercises the docker paths that talk to the daemon through the
// API client. It needs a running docker daemon and skips without one.
func TestDockerAPI(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	r, err := Detect(ctx)
	if err != nil || r.Name() != "docker" {
		t.Skip("no docker daemon reachable; skipping")
	}
	t.Logf("socket=%s", r.(*Docker).socket)

	rootless, err := r.IsRootless(ctx)
	require.NoError(t, err)
	t.Logf("rootless=%v", rootless)

	const net = "k0da-smoke-net"
	require.NoError(t, r.EnsureNetwork(ctx, net))
	require.NoError(t, r.EnsureNetwork(ctx, net), "must be idempotent")

	const name = "k0da-smoke-ctr"
	_ = r.RemoveContainer(ctx, name)
	_, err = r.RunContainer(ctx, RunContainerOptions{
		Name:    name,
		Image:   "alpine:3.20",
		Args:    []string{"sleep", "300"},
		Network: net,
		Labels:  map[string]string{"k0da.smoke": "true"},
		Publish: []PortSpec{{ContainerPort: 8080, Protocol: "tcp"}},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = r.RemoveContainer(context.Background(), name)
	})

	ip, port, err := r.GetPortMapping(ctx, name, 8080, "tcp")
	require.NoError(t, err)
	require.NotZero(t, port)
	t.Logf("port mapping %s:%d", ip, port)

	_, _, err = r.GetPortMapping(ctx, name, 9999, "tcp")
	require.ErrorContains(t, err, "no host binding")

	out, code, err := r.ExecInContainer(ctx, name, []string{"echo", "hello"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Equal(t, "hello\n", out)

	out, code, err = r.ExecInContainer(ctx, name, []string{"sh", "-c", "echo oops >&2; exit 3"})
	require.NoError(t, err)
	require.Equal(t, 3, code, "exit code must survive")
	require.Equal(t, "oops\n", out, "stderr must be captured")

	src := filepath.Join(t.TempDir(), "payload.txt")
	require.NoError(t, os.WriteFile(src, []byte("copied-content"), 0o644))
	require.NoError(t, r.CopyToContainer(ctx, name, src, "/tmp/inside.txt"))
	out, code, err = r.ExecInContainer(ctx, name, []string{"cat", "/tmp/inside.txt"})
	require.NoError(t, err)
	require.Equal(t, 0, code)
	require.Equal(t, "copied-content", out)

	tarPath := filepath.Join(t.TempDir(), "image.tar")
	require.NoError(t, r.SaveImageToTar(ctx, "alpine:3.20", tarPath))
	f, err := os.Open(tarPath)
	require.NoError(t, err)
	defer f.Close()
	_, err = tar.NewReader(f).Next()
	require.NoError(t, err, "saved archive must be a readable tar")
	fi, err := f.Stat()
	require.NoError(t, err)
	require.Greater(t, fi.Size(), int64(1<<20))
	t.Logf("saved image %d bytes", fi.Size())

	list, err := r.ListContainersByLabel(ctx, map[string]string{"k0da.smoke": "true"}, false)
	require.NoError(t, err)
	require.Len(t, list, 1)
}
