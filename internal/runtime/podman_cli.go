package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Podman implements Runtime using the podman CLI only (no cgo, no gpgme).
type Podman struct {
	name       string
	socket     string
	identity   string
	connection string
}

func NewPodmanRuntime(ctx context.Context, socket string, identity string) (*Podman, error) {
	// Optional explicit connection name (e.g., podman-machine-default-root)
	connName := strings.TrimSpace(os.Getenv("K0DA_PODMAN_CONNECTION"))
	if connName == "" {
		if name, ok := findPreferredPodmanConnection(ctx); ok {
			connName = name
		}
	}

	// Validate connectivity via CLI. Respect connection if provided, else use CONTAINER_HOST when given.
	args := []string{"version", "--format", "{{.Version}}"}
	if connName != "" {
		args = append([]string{"--connection", connName}, args...)
	}
	cmd := exec.CommandContext(ctx, "podman", args...)
	env := os.Environ()
	if connName == "" && socket != "" {
		env = append(env, "CONTAINER_HOST="+socket)
	}
	// identity (ssh key) handled by CONTAINER_SSHKEY if using ssh://
	if connName == "" && strings.HasPrefix(socket, "ssh://") && identity != "" && os.Getenv("CONTAINER_SSHKEY") == "" {
		env = append(env, "CONTAINER_SSHKEY="+identity)
	}
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return nil, fmt.Errorf("podman CLI not available or unreachable: %s", strings.TrimSpace(string(out)))
	}
	return &Podman{name: "podman", socket: socket, identity: identity, connection: connName}, nil
}

func (p *Podman) Name() string { return p.name }

func (p *Podman) withEnv(cmd *exec.Cmd) *exec.Cmd {
	env := os.Environ()
	if p.connection == "" && p.socket != "" {
		env = append(env, "CONTAINER_HOST="+p.socket)
	}
	if p.connection == "" && strings.HasPrefix(p.socket, "ssh://") && strings.TrimSpace(p.identity) != "" && os.Getenv("CONTAINER_SSHKEY") == "" {
		env = append(env, "CONTAINER_SSHKEY="+p.identity)
	}
	cmd.Env = env
	return cmd
}

func (p *Podman) argsWithConnection(args []string) []string {
	if strings.TrimSpace(p.connection) != "" {
		return append([]string{"--connection", p.connection}, args...)
	}
	return args
}

func (p *Podman) RunContainer(ctx context.Context, opts RunContainerOptions) (string, error) {
	args := []string{"run", "-d", "--restart", "unless-stopped"}
	if strings.TrimSpace(opts.Name) != "" {
		args = append(args, "--name", opts.Name)
	}
	if strings.TrimSpace(opts.Hostname) != "" {
		args = append(args, "--hostname", opts.Hostname)
	}
	if opts.Privileged {
		args = append(args, "--privileged")
	}
	if len(opts.Env) > 0 {
		for _, e := range opts.Env {
			args = append(args, "-e", e.Name+"="+e.Value)
		}
	}
	if len(opts.Labels) > 0 {
		for k, v := range opts.Labels {
			args = append(args, "--label", k+"="+v)
		}
	}
	if len(opts.Mounts) > 0 {
		for _, m := range opts.Mounts {
			// For both bind and volume, -v source:target[:options]
			entry := m.Source + ":" + m.Target
			if len(m.Options) > 0 {
				entry += ":" + strings.Join(m.Options, ",")
			}
			args = append(args, "-v", entry)
		}
	}
	if len(opts.Tmpfs) > 0 {
		for path, opt := range opts.Tmpfs {
			if strings.TrimSpace(opt) != "" {
				args = append(args, "--tmpfs", path+":"+opt)
			} else if strings.TrimSpace(path) != "" {
				args = append(args, "--tmpfs", path)
			}
		}
	}
	if len(opts.SecurityOpt) > 0 {
		for _, s := range opts.SecurityOpt {
			args = append(args, "--security-opt", s)
		}
	}
	if strings.TrimSpace(opts.Network) != "" {
		args = append(args, "--network", opts.Network)
	}
	if len(opts.Publish) > 0 {
		for _, ps := range opts.Publish {
			proto := strings.ToLower(ps.Protocol)
			if proto == "" {
				proto = "tcp"
			}
			// If host port is zero, request dynamic host port assignment
			if ps.HostPort == 0 {
				args = append(args, "-p", fmt.Sprintf("%d/%s", ps.ContainerPort, proto))
				continue
			}
			// hostIP optional
			prefix := ""
			if strings.TrimSpace(ps.HostIP) != "" {
				prefix = ps.HostIP + ":"
			}
			args = append(args, "-p", fmt.Sprintf("%s%d:%d/%s", prefix, ps.HostPort, ps.ContainerPort, proto))
		}
	}
	// Image then command args
	if strings.TrimSpace(opts.Image) == "" {
		return "", errors.New("image is required")
	}
	args = append(args, opts.Image)
	if len(opts.Args) > 0 {
		args = append(args, opts.Args...)
	}

	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection(args)...))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("podman run failed: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (p *Podman) ContainerExists(ctx context.Context, name string) (bool, error) {
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection([]string{"inspect", "-t", "container", name})...))
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() != 0 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (p *Podman) ContainerIsRunning(ctx context.Context, name string) (bool, error) {
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection([]string{"inspect", "-t", "container", name, "--format", "{{.State.Running}}"})...))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "true", nil
}

func (p *Podman) StopContainer(ctx context.Context, name string) error {
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection([]string{"stop", name})...))
	_, err := cmd.CombinedOutput()
	return err
}

func (p *Podman) RemoveContainer(ctx context.Context, name string) error {
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection([]string{"rm", "-f", name})...))
	_, err := cmd.CombinedOutput()
	return err
}

func (p *Podman) ExecInContainer(ctx context.Context, name string, command []string) (string, int, error) {
	args := append([]string{"exec", name}, command...)
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection(args)...))
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(out), ee.ExitCode(), nil
		}
		return string(out), 1, err
	}
	return string(out), 0, nil
}

func (p *Podman) GetPortMapping(ctx context.Context, name string, containerPort int, protocol string) (string, int, error) {
	proto := strings.ToLower(protocol)
	if proto == "" {
		proto = "tcp"
	}
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection([]string{"port", name, fmt.Sprintf("%d/%s", containerPort, proto)})...))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", 0, fmt.Errorf("podman port failed: %s", strings.TrimSpace(string(out)))
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return "", 0, fmt.Errorf("port mapping not found")
	}
	idx := strings.LastIndex(s, ":")
	if idx == -1 || idx+1 >= len(s) {
		return "", 0, fmt.Errorf("unexpected port output: %s", s)
	}
	host := strings.TrimSpace(s[:idx])
	portStr := strings.TrimSpace(s[idx+1:])
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	n, _ := strconv.Atoi(portStr)
	return host, n, nil
}

func (p *Podman) VolumeExists(ctx context.Context, name string) (bool, error) {
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection([]string{"volume", "inspect", name})...))
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() != 0 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (p *Podman) RemoveVolume(ctx context.Context, name string) error {
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection([]string{"volume", "rm", "-f", name})...))
	_, err := cmd.CombinedOutput()
	return err
}

func (p *Podman) ListContainersByLabel(ctx context.Context, selector map[string]string, includeStopped bool) ([]ContainerInfo, error) {
	args := []string{"ps", "--format", "json"}
	if includeStopped {
		args = append(args, "-a")
	}
	for k, v := range selector {
		args = append(args, "--filter", fmt.Sprintf("label=%s=%s", k, v))
	}
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection(args)...))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("podman ps failed: %s", strings.TrimSpace(string(out)))
	}
	var arr []map[string]any
	if err := json.Unmarshal(out, &arr); err != nil {
		return nil, err
	}
	outList := make([]ContainerInfo, 0, len(arr))
	for _, m := range arr {
		ci := ContainerInfo{}
		if v, ok := m["ID"].(string); ok {
			ci.ID = v
		} else if v, ok := m["Id"].(string); ok {
			ci.ID = v
		}
		if v, ok := m["Names"].([]any); ok && len(v) > 0 {
			if s, ok2 := v[0].(string); ok2 {
				ci.Name = strings.TrimPrefix(strings.TrimSpace(s), "/")
			}
		} else if s, ok := m["Name"].(string); ok {
			ci.Name = strings.TrimPrefix(strings.TrimSpace(s), "/")
		}
		if s, ok := m["Image"].(string); ok {
			ci.Image = s
		}
		if s, ok := m["Status"].(string); ok {
			ci.Status = s
		}
		if labels, ok := m["Labels"].(map[string]any); ok {
			ci.Labels = map[string]string{}
			for k, v := range labels {
				if vs, ok2 := v.(string); ok2 {
					ci.Labels[k] = vs
				}
			}
		}
		// Ports formatting
		if ports, ok := m["Ports"].([]any); ok {
			var b strings.Builder
			for i, pi := range ports {
				pm, ok2 := pi.(map[string]any)
				if !ok2 {
					continue
				}
				if i > 0 {
					b.WriteString(", ")
				}
				hostIP := "0.0.0.0"
				if hip, ok3 := pm["HostIp"].(string); ok3 && hip != "" {
					hostIP = hip
				}
				var hostPort, contPort int
				if hp, ok3 := pm["HostPort"].(float64); ok3 {
					hostPort = int(hp)
				}
				if cp, ok3 := pm["ContainerPort"].(float64); ok3 {
					contPort = int(cp)
				}
				proto := "tcp"
				if pr, ok3 := pm["Protocol"].(string); ok3 && pr != "" {
					proto = strings.ToLower(pr)
				}
				if hostPort == 0 {
					fmt.Fprintf(&b, "%s:%d->%d/%s", hostIP, contPort, contPort, proto)
				} else {
					fmt.Fprintf(&b, "%s:%d->%d/%s", hostIP, hostPort, contPort, proto)
				}
			}
			ci.Ports = b.String()
		}
		// Created timestamp if available
		if ts, ok := m["Created"].(float64); ok {
			ci.Created = int64(ts)
		}
		outList = append(outList, ci)
	}
	return outList, nil
}

func (p *Podman) CopyToContainer(ctx context.Context, name string, srcPath string, dstPath string) error {
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection([]string{"cp", srcPath, name + ":" + dstPath})...))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman cp failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (p *Podman) SaveImageToTar(ctx context.Context, imageRef string, tarPath string) error {
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection([]string{"save", "-o", tarPath, imageRef})...))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman save failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureNetwork ensures a user-defined network exists with the given name.
func (p *Podman) EnsureNetwork(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	// podman network inspect <name>
	cmd := p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection([]string{"network", "inspect", name})...))
	if err := cmd.Run(); err == nil {
		return nil
	}
	// create attachable bridge network by default
	cmd = p.withEnv(exec.CommandContext(ctx, "podman", p.argsWithConnection([]string{"network", "create", name})...))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman network create failed: %s", strings.TrimSpace(string(out)))
	}
	_ = out
	return nil
}

// findPreferredPodmanConnection returns a rootful or default connection name from
// `podman system connection list --format json`.
func findPreferredPodmanConnection(ctx context.Context) (string, bool) {
	cmd := exec.CommandContext(ctx, "podman", "system", "connection", "list", "--format", "json")
	out, err := cmd.CombinedOutput()
	if err != nil || len(out) == 0 {
		return "", false
	}
	var arr []map[string]any
	if err := json.Unmarshal(out, &arr); err != nil || len(arr) == 0 {
		return "", false
	}
	// 1) Prefer an entry that clearly indicates root (URI starts with ssh://root@ or name contains "root")
	for _, m := range arr {
		name, _ := m["Name"].(string)
		uri, _ := m["URI"].(string)
		if strings.HasPrefix(strings.ToLower(uri), "ssh://root@") || strings.Contains(strings.ToLower(name), "root") {
			return name, true
		}
	}
	// 2) Prefer default/active/current
	for _, m := range arr {
		name, _ := m["Name"].(string)
		for _, key := range []string{"Default", "Active", "Current"} {
			if v, ok := m[key]; ok {
				switch x := v.(type) {
				case bool:
					if x {
						return name, true
					}
				case string:
					if strings.TrimSpace(x) == "*" {
						return name, true
					}
				}
			}
		}
	}
	// 3) Fallback: first entry
	if name, _ := arr[0]["Name"].(string); name != "" {
		return name, true
	}
	return "", false
}
