package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DetectOptions struct {
	Runtime        string
	SocketOverride string
}

func getenv(keys ...string) (string, bool) {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v, true
		}
	}
	return "", false
}

func normalizeSocket(s string) string {
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "/") {
		return "unix://" + s
	}
	if strings.HasPrefix(s, "unix:/") && !strings.HasPrefix(s, "unix:///") {
		return "unix://" + strings.TrimPrefix(s, "unix:")
	}
	return s
}

// tryPodmanMacTmpdirSocket checks $TMPDIR/podman/podman-machine-default.sock
// and returns a unix:// URI if present and reachable.
func tryPodmanMacTmpdirSocket() string {
	tmp := os.Getenv("TMPDIR")
	if tmp == "" {
		return ""
	}
	p := filepath.Join(tmp, "podman", "podman-machine-default.sock")
	if _, err := os.Stat(p); err == nil {
		// Probe reachability (connection refused -> skip)
		conn, err := net.DialTimeout("unix", p, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return "unix://" + p
		}
	}
	return ""
}

// getDockerSocketCandidates returns a list of potential Docker socket paths
// ordered by preference (most common/reliable first).
func getDockerSocketCandidates() []string {
	home := os.Getenv("HOME")
	candidates := []string{"/var/run/docker.sock"}

	// Add user-specific candidates if HOME is available
	if home != "" {
		candidates = append(candidates, []string{
			filepath.Join(home, ".colima", "docker.sock"),
			filepath.Join(home, ".colima", "default", "docker.sock"),
			filepath.Join(home, ".orbstack", "run", "docker.sock"),
			filepath.Join(home, ".lima", "default", "sock", "docker.sock"),
			filepath.Join(home, "Library", "Containers", "com.docker.docker", "Data", "vms", "0", "docker.sock"),
			filepath.Join(home, ".rd", "docker.sock"),
			filepath.Join(home, ".local", "share", "containers", "podman", "machine", "podman-machine-default", "podman.sock"),
		}...)
	}

	return candidates
}

// tryDockerSocketCandidates checks all Docker socket candidates and returns
// the first one that exists and is reachable, or empty string if none found.
func tryDockerSocketCandidates() string {
	candidates := getDockerSocketCandidates()

	for _, socketPath := range candidates {
		if _, err := os.Stat(socketPath); err == nil {
			// Probe reachability (connection refused -> skip)
			conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				return "unix://" + socketPath
			}
		}
	}
	return ""
}

// tryPodmanConnectionList queries `podman system connection list --format json`
// and returns the URI and Identity of the best connection (prefer rootful), or "" if not found.
func tryPodmanConnectionList() (string, string) {
	cmd := exec.Command("podman", "system", "connection", "list", "--format", "json")
	out, err := cmd.CombinedOutput()
	if err != nil || len(out) == 0 {
		return "", ""
	}
	var arr []map[string]any
	if err := json.Unmarshal(out, &arr); err != nil || len(arr) == 0 {
		return "", ""
	}
	// Prefer rootful connection
	for _, m := range arr {
		uri, _ := m["URI"].(string)
		id, _ := m["Identity"].(string)
		if strings.HasPrefix(uri, "ssh://root@") {
			return uri, id
		}
	}
	// Otherwise prefer default/active/current
	pick := -1
	for i, m := range arr {
		for _, key := range []string{"Default", "Active", "Current"} {
			if v, ok := m[key]; ok {
				switch x := v.(type) {
				case bool:
					if x {
						pick = i
					}
				case string:
					if strings.TrimSpace(x) == "*" {
						pick = i
					}
				}
			}
		}
		if pick != -1 {
			break
		}
	}
	if pick == -1 {
		pick = 0
	}
	if pick >= 0 && pick < len(arr) {
		uri, _ := arr[pick]["URI"].(string)
		id, _ := arr[pick]["Identity"].(string)
		return uri, id
	}
	return "", ""
}

type machineInfo struct {
	Name    string
	Rootful bool
}

func podmanMachineIsRootful() (bool, bool) {
	cmd := exec.Command("podman", "machine", "inspect")
	out, err := cmd.CombinedOutput()
	if err != nil || len(out) == 0 {
		return false, false
	}
	var arr []map[string]any
	if err := json.Unmarshal(out, &arr); err != nil || len(arr) == 0 {
		return false, false
	}
	if v, ok := arr[0]["Rootful"].(bool); ok {
		return v, true
	}
	return false, false
}

func Detect(ctx context.Context, opts DetectOptions) (Runtime, error) {
	runtime := strings.ToLower(strings.TrimSpace(opts.Runtime))
	if runtime == "" {
		if v, ok := getenv("K0DA_RUNTIME", "K0DA_BACKEND"); ok {
			runtime = strings.ToLower(v)
		}
	}
	socket := strings.TrimSpace(opts.SocketOverride)
	if socket == "" {
		if v, ok := getenv("K0DA_SOCKET"); ok {
			socket = v
		}
	}
	// If runtime is docker/empty and DOCKER_HOST set, prefer it when socket still empty
	if socket == "" && (runtime == "docker" || runtime == "") {
		if v, ok := getenv("DOCKER_HOST"); ok {
			socket = v
		}
	}
	identity := ""
	// If socket still empty, guess per runtime
	if socket == "" {
		switch runtime {
		case "docker", "":
			// Try all Docker socket candidates
			if s := tryDockerSocketCandidates(); s != "" {
				socket = s
			} else {
				// Fallback to default Docker socket
				socket = "unix:///var/run/docker.sock"
			}
		case "podman":
			// First, prefer connection list (rootful if available)
			u, id := tryPodmanConnectionList()
			if u != "" {
				socket, identity = u, id
			} else {
				// Otherwise try standard sockets
				if rd, ok := getenv("XDG_RUNTIME_DIR"); ok {
					p := filepath.Join(rd, "podman", "podman.sock")
					if _, err := os.Stat(p); err == nil {
						socket = "unix://" + p
					}
				}
				if socket == "" {
					cands := []string{"/run/podman/podman.sock", "/var/run/podman/podman.sock"}
					for _, p := range cands {
						if _, err := os.Stat(p); err == nil {
							socket = "unix://" + p
							break
						}
					}
				}
				// On macOS, $TMPDIR unix socket as last resort (only if reachable)
				if socket == "" {
					if s := tryPodmanMacTmpdirSocket(); s != "" {
						socket = s
					}
				}
			}
		}
	}
	// Normalize final socket before use
	socket = normalizeSocket(socket)

	// If using Podman on macOS and machine is rootless but we selected a rootless connection, check for rootful alternative
	if runtime == "podman" {
		if isRootful, ok := podmanMachineIsRootful(); ok && !isRootful {
			// Try to find rootful connection
			u2, id2 := tryPodmanConnectionList()
			if strings.HasPrefix(u2, "ssh://root@") {
				socket, identity = u2, id2
			} else {
				return nil, fmt.Errorf("podman machine is rootless; please run 'podman machine set --rootful' and restart, or set K0DA_RUNTIME=docker")
			}
		}
	}

	if runtime != "" {
		switch runtime {
		case "docker":
			return NewDockerRuntime(ctx, socket)
		case "podman":
			return NewPodmanRuntime(ctx, socket, identity)
		case "containerd":
			return nil, fmt.Errorf("containerd backend not implemented yet")
		default:
			return nil, fmt.Errorf("unknown runtime: %s", runtime)
		}
	}
	// Try Docker with any available socket first if no socket was set
	if socket == "" {
		socket = tryDockerSocketCandidates()
	}

	if b, err := NewDockerRuntime(ctx, socket); err == nil {
		return b, nil
	}
	if b, err := NewPodmanRuntime(ctx, socket, identity); err == nil {
		return b, nil
	}
	return nil, fmt.Errorf("no supported container runtime detected; set K0DA_RUNTIME=docker|podman or configure socket")
}
