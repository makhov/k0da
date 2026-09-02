package runtime

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	imageTypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	dockerClient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

type Docker struct {
	cli    *dockerClient.Client
	name   string
	socket string
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
	return &Docker{cli: client, name: "docker", socket: socket}, nil
}

func (d *Docker) Name() string { return d.name }

// IsRootless reports whether the Docker daemon runs in rootless mode.
// Both dockerd and podman's Docker-compatible API advertise this as a
// "name=rootless" entry in SecurityOptions.
func (d *Docker) IsRootless(ctx context.Context) (bool, error) {
	info, err := d.cli.Info(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to query daemon info: %w", err)
	}
	for _, opt := range info.SecurityOptions {
		if strings.Contains(opt, "name=rootless") {
			return true, nil
		}
	}
	return false, nil
}

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
	if hostConfig.Ulimits == nil {
		hostConfig.Ulimits = []*container.Ulimit{}
	}
	hostConfig.Ulimits = append(hostConfig.Ulimits, &container.Ulimit{Name: "memlock", Soft: -1, Hard: -1})

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

	// Ensure container restarts after daemon restart
	hostConfig.RestartPolicy = container.RestartPolicy{Name: "always"}

	networking := &network.NetworkingConfig{}
	if strings.TrimSpace(opts.Network) != "" {
		networking.EndpointsConfig = map[string]*network.EndpointSettings{
			opts.Network: {},
		}
	}

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
	created, err := d.cli.ContainerExecCreate(ctx, name, container.ExecOptions{
		Cmd:          command,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return "", 1, err
	}
	att, err := d.cli.ContainerExecAttach(ctx, created.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", 1, err
	}
	defer att.Close()

	// The exec runs without a TTY, so its stream is multiplexed. Fold both
	// channels into one buffer, the way `docker exec` output reads.
	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, att.Reader); err != nil {
		return buf.String(), 1, err
	}
	insp, err := d.cli.ContainerExecInspect(ctx, created.ID)
	if err != nil {
		return buf.String(), 1, err
	}
	return buf.String(), insp.ExitCode, nil
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
	return "", 0, fmt.Errorf("no host binding for %s in container %s", key, name)
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

// CopyToContainer copies a local file into the container at dstPath.
func (d *Docker) CopyToContainer(ctx context.Context, name string, srcPath string, dstPath string) error {
	content, err := tarFile(srcPath, filepath.Base(dstPath))
	if err != nil {
		return err
	}
	defer content.Close()
	if err := d.cli.CopyToContainer(ctx, name, filepath.Dir(dstPath), content, container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("failed to copy %s into %s: %w", srcPath, name, err)
	}
	return nil
}

// SaveImageToTar saves a local Docker image into a tar archive
func (d *Docker) SaveImageToTar(ctx context.Context, imageRef string, tarPath string) error {
	rc, err := d.cli.ImageSave(ctx, []string{imageRef})
	if err != nil {
		return fmt.Errorf("failed to save image %s: %w", imageRef, err)
	}
	defer rc.Close()
	f, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, rc); err != nil {
		_ = f.Close()
		return fmt.Errorf("failed to write image archive %s: %w", tarPath, err)
	}
	return f.Close()
}

// tarFile streams a single regular file as a tar archive under the given name.
// The docker API only accepts tar streams for container copies, and k0da only
// ever copies one image archive at a time. Streaming keeps memory bounded:
// image archives routinely run to hundreds of megabytes.
func tarFile(srcPath, name string) (io.ReadCloser, error) {
	fi, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("cannot copy %s into container: not a regular file", srcPath)
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = f.Close() }()
		tw := tar.NewWriter(pw)
		err := tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     name,
			Mode:     int64(fi.Mode().Perm()),
			Size:     fi.Size(),
			ModTime:  fi.ModTime(),
		})
		if err == nil {
			_, err = io.Copy(tw, f)
		}
		if err == nil {
			err = tw.Close()
		}
		_ = pw.CloseWithError(err)
	}()
	return pr, nil
}

func atoiSafe(s string) int { n, _ := strconv.Atoi(s); return n }

func formatPorts(ports []container.Port) string {
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

// EnsureNetwork ensures a user-defined bridge network exists with the given name.
func (d *Docker) EnsureNetwork(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return nil
	}
	_, err := d.cli.NetworkInspect(ctx, name, network.InspectOptions{})
	if err == nil {
		return nil
	}
	if !dockerClient.IsErrNotFound(err) {
		return fmt.Errorf("failed to inspect network %q: %w", name, err)
	}
	_, err = d.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:     "bridge",
		Attachable: true,
		Labels: map[string]string{
			"k0da.network":      "true",
			"k0da.network.name": name,
		},
	})
	if err != nil {
		// Another creator may have won the race between inspect and create.
		if _, ierr := d.cli.NetworkInspect(ctx, name, network.InspectOptions{}); ierr == nil {
			return nil
		}
		return fmt.Errorf("failed to create network %q: %w", name, err)
	}
	return nil
}
