package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Detect returns the first container runtime that answers on one of its
// candidate endpoints.
//
// K0DA_RUNTIME pins the runtime to docker or podman, K0DA_SOCKET pins the
// endpoint. Without them k0da probes docker first, then podman. Every endpoint
// is validated by connecting to it, so a candidate that is stale, belongs to a
// stopped VM, or uses a transport the API client cannot dial is skipped in
// favour of the next one.
func Detect(ctx context.Context) (Runtime, error) {
	name := strings.ToLower(getenv("K0DA_RUNTIME", "K0DA_BACKEND"))
	socket := getenv("K0DA_SOCKET")

	switch name {
	case "docker":
		return detectDocker(ctx, socket)
	case "podman":
		return NewPodmanRuntime(ctx, socket)
	case "containerd":
		return nil, errors.New("containerd runtime is not implemented yet")
	case "":
		// Fall through to probing.
	default:
		return nil, fmt.Errorf("unknown runtime %q: expected docker or podman", name)
	}

	if r, err := detectDocker(ctx, socket); err == nil {
		return r, nil
	}
	if r, err := NewPodmanRuntime(ctx, socket); err == nil {
		return r, nil
	}
	return nil, errors.New("no supported container runtime detected. Set K0DA_RUNTIME=docker|podman and K0DA_SOCKET=<socket> to override detection")
}

// detectDocker connects to the first docker endpoint whose daemon answers.
func detectDocker(ctx context.Context, override string) (Runtime, error) {
	var errs []error
	for _, host := range dockerCandidates(ctx, override) {
		d, err := NewDockerRuntime(ctx, host)
		if err == nil {
			return d, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", host, err))
	}
	return nil, fmt.Errorf("no reachable docker daemon: %w", errors.Join(errs...))
}

// dockerCandidates lists docker endpoints in the precedence the docker CLI
// itself uses — DOCKER_HOST, then the active CLI context — followed by the
// socket paths of the desktop runtimes that expose a docker-compatible API.
// On a machine with several runtimes installed /var/run/docker.sock often
// belongs to a different one than the context the user actually works in, so
// the context endpoint has to win over it.
func dockerCandidates(ctx context.Context, override string) []string {
	if override != "" {
		return []string{override}
	}
	var out []string
	if h := getenv("DOCKER_HOST"); h != "" {
		out = append(out, h)
	}
	if h := dockerContextEndpoint(ctx); h != "" {
		out = append(out, h)
	}
	if runtime.GOOS == "windows" {
		return dedupe(append(out, "npipe:////./pipe/docker_engine"))
	}
	out = append(out, "unix:///var/run/docker.sock")
	if home, err := os.UserHomeDir(); err == nil {
		for _, parts := range dockerHomeSockets {
			out = append(out, "unix://"+filepath.Join(home, filepath.Join(parts...)))
		}
	}
	return dedupe(out)
}

// dockerHomeSockets are docker-compatible sockets that the common desktop
// runtimes place under the user's home directory, relative to it.
var dockerHomeSockets = [][]string{
	{".docker", "run", "docker.sock"},                                                 // Docker Desktop
	{"Library", "Containers", "com.docker.docker", "Data", "vms", "0", "docker.sock"}, // Docker Desktop, older macOS layout
	{".colima", "docker.sock"},                                                        // Colima
	{".colima", "default", "docker.sock"},                                             // Colima, named profile layout
	{".orbstack", "run", "docker.sock"},                                               // OrbStack
	{".rd", "docker.sock"},                                                            // Rancher Desktop
	{".lima", "default", "sock", "docker.sock"},                                       // Lima
	{".local", "share", "containers", "podman", "machine", "podman-machine-default", "podman.sock"}, // podman's docker-compatible API
}

// dockerContextEndpoint returns the docker endpoint of the CLI's active
// context, so k0da talks to the same daemon as the user's `docker` command.
// It returns "" when the docker CLI is absent or has no usable context.
func dockerContextEndpoint(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getenv returns the first non-empty value among keys.
func getenv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := in[:0]
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
