package runtime

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetenv(t *testing.T) {
	t.Setenv("K0DA_A", "")
	t.Setenv("K0DA_B", "  podman  ")
	require.Equal(t, "podman", getenv("K0DA_A", "K0DA_B"))
	require.Equal(t, "", getenv("K0DA_MISSING"))
}

func TestDockerCandidates_OverrideWins(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///from/env.sock")
	require.Equal(t,
		[]string{"unix:///override.sock"},
		dockerCandidates(context.Background(), "unix:///override.sock"))
}

func TestDockerCandidates_EnvBeforeDefaults(t *testing.T) {
	t.Setenv("DOCKER_HOST", "unix:///from/env.sock")
	got := dockerCandidates(context.Background(), "")
	require.Equal(t, "unix:///from/env.sock", got[0])
	require.Contains(t, got, "unix:///var/run/docker.sock")
	require.Greater(t, len(got), 1)
}

func TestDockerCandidates_NoDuplicates(t *testing.T) {
	// A DOCKER_HOST equal to the default must not be probed twice.
	t.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
	got := dockerCandidates(context.Background(), "")
	require.Equal(t, "unix:///var/run/docker.sock", got[0])

	seen := map[string]bool{}
	for _, c := range got {
		require.False(t, seen[c], "duplicate candidate %s", c)
		seen[c] = true
	}
}

func TestDetect_UnknownRuntime(t *testing.T) {
	t.Setenv("K0DA_RUNTIME", "rkt")
	_, err := Detect(context.Background())
	require.ErrorContains(t, err, `unknown runtime "rkt"`)
}

func TestDetect_Containerd(t *testing.T) {
	t.Setenv("K0DA_RUNTIME", "containerd")
	_, err := Detect(context.Background())
	require.ErrorContains(t, err, "not implemented")
}

func TestTarFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "image.tar")
	require.NoError(t, os.WriteFile(src, []byte("payload"), 0o644))

	rc, err := tarFile(src, "renamed.tar")
	require.NoError(t, err)
	defer rc.Close()

	tr := tar.NewReader(rc)
	hdr, err := tr.Next()
	require.NoError(t, err)
	require.Equal(t, "renamed.tar", hdr.Name)
	require.Equal(t, int64(len("payload")), hdr.Size)

	body, err := io.ReadAll(tr)
	require.NoError(t, err)
	require.Equal(t, "payload", string(body))

	_, err = tr.Next()
	require.Equal(t, io.EOF, err)
}

func TestTarFile_RejectsDirectory(t *testing.T) {
	_, err := tarFile(t.TempDir(), "x")
	require.ErrorContains(t, err, "not a regular file")
}
