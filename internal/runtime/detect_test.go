package runtime

import (
	"os"
	"testing"
)

func TestNormalizeSocket(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"unix:///var/run/docker.sock", "unix:///var/run/docker.sock"},
		{"/var/run/docker.sock", "unix:///var/run/docker.sock"},
		{"unix:/var/run/docker.sock", "unix:///var/run/docker.sock"},
		{"ssh://root@host/run/podman/podman.sock", "ssh://root@host/run/podman/podman.sock"},
	}
	for _, c := range cases {
		if got := normalizeSocket(c.in); got != c.want {
			t.Fatalf("normalizeSocket(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestGetDockerSocketCandidates(t *testing.T) {
	// Test with empty HOME
	oldHome := os.Getenv("HOME")
	_ = os.Unsetenv("HOME")
	defer func() {
		if oldHome != "" {
			_ = os.Setenv("HOME", oldHome)
		}
	}()

	candidates := getDockerSocketCandidates()
	if len(candidates) != 1 {
		t.Errorf("Expected 1 candidate with empty HOME, got %d", len(candidates))
	}
	if candidates[0] != "/var/run/docker.sock" {
		t.Errorf("Expected first candidate to be '/var/run/docker.sock', got %q", candidates[0])
	}

	// Test with HOME set
	_ = os.Setenv("HOME", "/tmp")
	candidates = getDockerSocketCandidates()
	if len(candidates) != 8 {
		t.Errorf("Expected 8 candidates with HOME set, got %d", len(candidates))
	}

	// Check that we have the expected paths
	expectedPaths := []string{
		"/var/run/docker.sock",
		"/tmp/.colima/docker.sock",
		"/tmp/.colima/default/docker.sock",
		"/tmp/.orbstack/run/docker.sock",
		"/tmp/.lima/default/sock/docker.sock",
		"/tmp/Library/Containers/com.docker.docker/Data/vms/0/docker.sock",
		"/tmp/.rd/docker.sock",
		"/tmp/.local/share/containers/podman/machine/podman-machine-default/podman.sock",
	}
	for i, expectedPath := range expectedPaths {
		if i >= len(candidates) {
			t.Errorf("Expected candidate %d to be %q, but candidates list is too short", i, expectedPath)
			continue
		}
		if candidates[i] != expectedPath {
			t.Errorf("Expected candidate %d to be %q, got %q", i, expectedPath, candidates[i])
		}
	}
}
