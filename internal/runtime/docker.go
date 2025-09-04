package runtime

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	imageTypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	dockerClient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type Docker struct {
	cli  *dockerClient.Client
	name string
}

func NewDockerRuntime(ctx context.Context, socket string) (*Docker, error) {
	if socket == "" {
		return nil, fmt.Errorf("docker socket not specified")
	}
	client, err := dockerClient.NewClientWithOpts(dockerClient.WithHost(socket), dockerClient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	// Ping to verify connectivity
	_, err = client.Ping(ctx)
	if err != nil {
		return nil, err
	}
	return &Docker{cli: client, name: "docker"}, nil
}

func (d *Docker) Name() string { return d.name }

func (d *Docker) RunContainer(ctx context.Context, opts RunContainerOptions) (string, error) {
	// Ensure image exists locally; pull if missing
	if opts.Image != "" {
		rc, err := d.cli.ImagePull(ctx, opts.Image, imageTypes.PullOptions{})
		if err != nil {
			return "", err
		}
		if rc != nil {
			_, _ = io.Copy(io.Discard, rc)
			_ = rc.Close()
		}
	}

	config := &container.Config{
		Image:    opts.Image,
		Cmd:      opts.Args,
		Env:      opts.Env.ToOSStrings(),
		Labels:   opts.Labels,
		Hostname: opts.Hostname,
		Tty:      true,
	}

	hostConfig := &container.HostConfig{
		AutoRemove:  opts.AutoRemove,
		Privileged:  opts.Privileged,
		SecurityOpt: opts.SecurityOpt,
		Tmpfs:       opts.Tmpfs,
	}
	// Set ulimit memlock unlimited for k0s eBPF
	if hostConfig.Resources.Ulimits == nil {
		hostConfig.Resources.Ulimits = []*container.Ulimit{}
	}
	hostConfig.Resources.Ulimits = append(hostConfig.Resources.Ulimits, &container.Ulimit{Name: "memlock", Soft: -1, Hard: -1})

	// Use Mounts helper
	if len(opts.Mounts) > 0 {
		hostConfig.Binds = opts.Mounts.ToBinds()
	}

	// Port publishing
	if len(opts.Publish) > 0 {
		hostConfig.PortBindings = natPortBindings(opts.Publish)
		// Ensure corresponding exposed ports are set so inspect shows mappings
		if config.ExposedPorts == nil {
			config.ExposedPorts = nat.PortSet{}
		}
		for _, ps := range opts.Publish {
			proto := strings.ToLower(ps.Protocol)
			if proto == "" {
				proto = "tcp"
			}
			portKey, _ := nat.NewPort(proto, fmt.Sprintf("%d", ps.ContainerPort))
			config.ExposedPorts[portKey] = struct{}{}
		}
	}

	networking := &network.NetworkingConfig{}

	resp, err := d.cli.ContainerCreate(ctx, config, hostConfig, networking, nil, opts.Name)
	if err != nil {
		return "", err
	}

	if err := d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (d *Docker) ContainerExists(ctx context.Context, name string) (bool, error) {
	filtersArgs := filters.NewArgs(filters.Arg("name", fmt.Sprintf("^%s$", name)))
	c, err := d.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: filtersArgs})
	if err != nil {
		return false, err
	}
	return len(c) > 0, nil
}

func (d *Docker) ContainerIsRunning(ctx context.Context, name string) (bool, error) {
	filtersArgs := filters.NewArgs(filters.Arg("name", fmt.Sprintf("^%s$", name)))
	c, err := d.cli.ContainerList(ctx, container.ListOptions{All: false, Filters: filtersArgs})
	if err != nil {
		return false, err
	}
	return len(c) > 0, nil
}

func (d *Docker) StopContainer(ctx context.Context, name string) error {
	timeout := int((10 * time.Second).Seconds())
	return d.cli.ContainerStop(ctx, name, container.StopOptions{Timeout: &timeout})
}

func (d *Docker) RemoveContainer(ctx context.Context, name string) error {
	return d.cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})
}

func (d *Docker) ExecInContainer(ctx context.Context, name string, command []string) (string, int, error) {
	// Fallback to docker CLI to avoid API type drift
	args := append([]string{"exec", name}, command...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Try to get exit code
		if ee, ok := err.(*exec.ExitError); ok {
			return string(out), ee.ExitCode(), nil
		}
		return string(out), 1, err
	}
	return string(out), 0, nil
}

func (d *Docker) GetPortMapping(ctx context.Context, name string, containerPort int, protocol string) (string, int, error) {
	insp, err := d.cli.ContainerInspect(ctx, name)
	if err != nil {
		return "", 0, err
	}
	proto := strings.ToLower(protocol)
	key := fmt.Sprintf("%d/%s", containerPort, proto)
	for p, bindings := range insp.NetworkSettings.Ports {
		if string(p) == key && len(bindings) > 0 {
			return bindings[0].HostIP, atoiSafe(bindings[0].HostPort), nil
		}
	}
	// Fallback to docker CLI: docker port <name> <port>/<proto>
	args := []string{"port", name, fmt.Sprintf("%d/%s", containerPort, proto)}
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		s := strings.TrimSpace(string(out))
		if s != "" {
			idx := strings.LastIndex(s, ":")
			if idx != -1 && idx+1 < len(s) {
				host := s[:idx]
				portStr := s[idx+1:]
				if host == "" {
					host = "0.0.0.0"
				}
				host = strings.TrimPrefix(host, "[")
				host = strings.TrimSuffix(host, "]")
				return host, atoiSafe(portStr), nil
			}
		}
	}
	return "", 0, fmt.Errorf("port mapping not found")
}

func (d *Docker) VolumeExists(ctx context.Context, name string) (bool, error) {
	vols, err := d.cli.VolumeList(ctx, volume.ListOptions{Filters: filters.NewArgs(filters.Arg("name", name))})
	if err != nil {
		return false, err
	}
	return len(vols.Volumes) > 0, nil
}

func (d *Docker) RemoveVolume(ctx context.Context, name string) error {
	return d.cli.VolumeRemove(ctx, name, true)
}

func (d *Docker) ListContainersByLabel(ctx context.Context, selector map[string]string, includeStopped bool) ([]ContainerInfo, error) {
	f := filters.NewArgs()
	for k, v := range selector {
		f.Add("label", fmt.Sprintf("%s=%s", k, v))
	}
	list, err := d.cli.ContainerList(ctx, container.ListOptions{All: includeStopped, Filters: f})
	if err != nil {
		return nil, err
	}
	out := make([]ContainerInfo, 0, len(list))
	for _, c := range list {
		ci := ContainerInfo{
			ID:      c.ID,
			Name:    strings.TrimPrefix(strings.TrimPrefix(c.Names[0], "/"), "/"),
			Image:   c.Image,
			Status:  c.Status,
			Ports:   formatPorts(c.Ports),
			Created: c.Created,
			Labels:  c.Labels,
		}
		out = append(out, ci)
	}
	return out, nil
}

// CopyToContainer copies a local path into the container
func (d *Docker) CopyToContainer(ctx context.Context, name string, srcPath string, dstPath string) error {
	cmd := exec.CommandContext(ctx, "docker", "cp", srcPath, name+":"+dstPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker cp failed: %s", string(out))
	}
	return nil
}

// SaveImageToTar saves a local Docker image into a tar archive
func (d *Docker) SaveImageToTar(ctx context.Context, imageRef string, tarPath string) error {
	cmd := exec.CommandContext(ctx, "docker", "save", "-o", tarPath, imageRef)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker save failed: %s", string(out))
	}
	return nil
}

func atoiSafe(s string) int { n, _ := strconv.Atoi(s); return n }

func formatPorts(ports []dockerTypes.Port) string {
	var b strings.Builder
	for i, p := range ports {
		if i > 0 {
			b.WriteString(", ")
		}
		hostIP := p.IP
		if hostIP == "" {
			hostIP = "0.0.0.0"
		}
		if p.PublicPort == 0 {
			fmt.Fprintf(&b, "%s:%d->%d/%s", hostIP, p.PrivatePort, p.PrivatePort, p.Type)
		} else {
			fmt.Fprintf(&b, "%s:%d->%d/%s", hostIP, p.PublicPort, p.PrivatePort, p.Type)
		}
	}
	return b.String()
}

// natPortBindings converts our PortSpec to docker's types.
func natPortBindings(publish []PortSpec) nat.PortMap {
	m := nat.PortMap{}
	for _, ps := range publish {
		proto := strings.ToLower(ps.Protocol)
		if proto == "" {
			proto = "tcp"
		}
		portKey, _ := nat.NewPort(proto, fmt.Sprintf("%d", ps.ContainerPort))
		hostIP := ps.HostIP
		if hostIP == "" {
			hostIP = "0.0.0.0"
		}
		b := nat.PortBinding{HostIP: hostIP}
		if ps.HostPort != 0 {
			b.HostPort = fmt.Sprintf("%d", ps.HostPort)
		}
		m[portKey] = append(m[portKey], b)
	}
	return m
}
