package runtime

import (
	"os"
	"path/filepath"
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
	os.Unsetenv("HOME")
	defer func() {
		if oldHome != "" {
			os.Setenv("HOME", oldHome)
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
	os.Setenv("HOME", "/tmp")
	candidates = getDockerSocketCandidates()
	if len(candidates) != 7 {
		t.Errorf("Expected 7 candidates with HOME set, got %d", len(candidates))
	}

	// Check that we have the expected paths
	expectedPaths := []string{
		"/var/run/docker.sock",
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

func TestTryDockerSocketCandidates(t *testing.T) {
	// Test with empty HOME
	oldHome := os.Getenv("HOME")
	os.Unsetenv("HOME")
	defer func() {
		if oldHome != "" {
			os.Setenv("HOME", oldHome)
		}
	}()

	result := tryDockerSocketCandidates()
	// Should return empty string since no sockets exist in test environment
	if result != "" {
		t.Errorf("tryDockerSocketCandidates() with no sockets should return empty string, got %q", result)
	}

	// Test with HOME set but no socket files
	os.Setenv("HOME", "/tmp")
	result = tryDockerSocketCandidates()
	if result != "" {
		t.Errorf("tryDockerSocketCandidates() with no socket files should return empty string, got %q", result)
	}

	// Test with HOME set and socket file exists (but not reachable)
	home := "/tmp"
	os.Setenv("HOME", home)
	colimaDir := filepath.Join(home, ".colima", "default")
	if err := os.MkdirAll(colimaDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	defer os.RemoveAll(filepath.Join(home, ".colima"))

	socketPath := filepath.Join(colimaDir, "docker.sock")
	if err := os.WriteFile(socketPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create test socket file: %v", err)
	}

	result = tryDockerSocketCandidates()
	if result != "" {
		t.Errorf("tryDockerSocketCandidates() with unreachable socket should return empty string, got %q", result)
	}
}
