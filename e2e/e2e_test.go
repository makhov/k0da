package e2e

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func getBinaryPath(t *testing.T) string {
	// Prefer explicit K0DA_BIN, else PATH
	if p := os.Getenv("K0DA_BIN"); strings.TrimSpace(p) != "" {
		return p
	}
	if p, err := exec.LookPath("k0da"); err == nil {
		return p
	}
	t.Skip("K0DA_BIN not set and 'k0da' not found in PATH; skipping e2e")
	return ""
}

func runCmd(t *testing.T, bin string, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = os.Environ()
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = 1
		}
	}

	return buf.String(), code
}

func TestE2E_CreateListDelete_NoConfig(t *testing.T) {
	k0daBin := getBinaryPath(t)
	name := "k0da-e2e-no-config-" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", "")
	// Ensure cleanup
	t.Cleanup(func() {
		_, _ = runCmd(t, k0daBin, "delete", "--name", name)
	})

	if out, code := runCmd(t, k0daBin, "create", "--name", name, "--timeout", "120s"); code != 0 {
		t.Fatalf("create failed (%d):\n%s", code, out)
	}
	if out, code := runCmd(t, k0daBin, "list"); code != 0 {
		t.Fatalf("list failed (%d):\n%s", code, out)
	} else if !strings.Contains(out, name) {
		t.Fatalf("cluster %q not found in list:\n%s", name, out)
	}
	if out, code := runCmd(t, k0daBin, "delete", "--name", name); code != 0 {
		t.Fatalf("delete failed (%d):\n%s", code, out)
	}
}

func TestE2E_CreateListDelete_WithConfig(t *testing.T) {
	k0daBin := getBinaryPath(t)
	name := "k0da-e2e-with-config-" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", "")

	// Write minimal cluster config
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "cluster.yaml")
	if err := os.WriteFile(cfgPath, []byte(k0daConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Ensure cleanup
	t.Cleanup(func() {
		_, _ = runCmd(t, k0daBin, "delete", "--name", name)
	})

	if out, code := runCmd(t, k0daBin, "create", "-n", name, "-c", cfgPath); code != 0 {
		t.Fatalf("create (with config) failed (%d):\n%s", code, out)
	}
	if out, code := runCmd(t, k0daBin, "list"); code != 0 {
		t.Fatalf("list failed (%d):\n%s", code, out)
	} else if !strings.Contains(out, name) {
		t.Fatalf("cluster %q not found in list:\n%s", name, out)
	}
}

var k0daConfig = `
apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  nodes:
    - role: controller
      ports:
        - containerPort: 443
          hostPort: 32443
  k0s:
    version: v1.33.2-k0s.0
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: my-k0s-cluster
      spec:
        api:
          extraArgs:
            anonymous-auth: "true"
  options:
    wait:
      enabled: true
    timeout: 120s
`

func findHostContainerRuntime(t *testing.T) string {
	if _, err := exec.LookPath("docker"); err == nil {
		if out, code := runCmd(t, "docker", "version", "--format", "{{.Server.Version}}"); code == 0 && strings.TrimSpace(out) != "" {
			return "docker"
		}
	}
	if _, err := exec.LookPath("podman"); err == nil {
		if out, code := runCmd(t, "podman", "version", "--format", "{{.Version.Version}}"); code == 0 && strings.TrimSpace(out) != "" {
			return "podman"
		}
	}
	t.Skip("No docker or podman found on host; skipping image load e2e tests")
	return ""
}

func buildTestImage(t *testing.T, runtime string, tag string) {
	t.Helper()
	dir := t.TempDir()
	dockerfile := "FROM busybox:1.36.1\nCOPY e2e.txt /e2e.txt\nCMD [\"sh\", \"-c\", \"sleep 3600\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "e2e.txt"), []byte("ok"), 0644); err != nil {
		t.Fatalf("write e2e file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		t.Fatalf("write Dockerfile: %v", err)
	}
	switch runtime {
	case "docker":
		if out, code := runCmd(t, "docker", append([]string{"build", "-t", tag, dir})...); code != 0 {
			t.Skipf("docker build failed (%d):\n%s", code, out)
		}
	case "podman":
		if out, code := runCmd(t, "podman", append([]string{"build", "-t", tag, dir})...); code != 0 {
			t.Skipf("podman build failed (%d):\n%s", code, out)
		}
	}
}

func saveImageToTar(t *testing.T, runtime string, image string) string {
	t.Helper()
	tar := filepath.Join(t.TempDir(), "image.tar")
	switch runtime {
	case "docker":
		if out, code := runCmd(t, "docker", "save", "-o", tar, image); code != 0 {
			t.Fatalf("docker save failed (%d):\n%s", code, out)
		}
	case "podman":
		if out, code := runCmd(t, "podman", "save", "-o", tar, image); code != 0 {
			t.Fatalf("podman save failed (%d):\n%s", code, out)
		}
	}
	return tar
}

func getKubeconfigPath(t *testing.T) string {
	h, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("home dir: %v", err)
	}
	return filepath.Join(h, ".kube", "config")
}

func runKubectl(t *testing.T, kubeconfig string, contextName string, args ...string) (string, int) {
	t.Helper()
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Skip("kubectl not found; skipping e2e")
	}
	allArgs := append([]string{"--kubeconfig", kubeconfig, "--context", contextName}, args...)
	return runCmd(t, "kubectl", allArgs...)
}

func waitForPodReady(t *testing.T, kubeconfig string, contextName string, name string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, _ := runKubectl(t, kubeconfig, contextName, "get", "pod", name, "-o", "jsonpath={.status.phase}")
		phase := strings.TrimSpace(out)
		if phase == "Running" {
			return
		}
		time.Sleep(3 * time.Second)
	}
	t.Fatalf("pod %s not running within %s", name, timeout)
}

func TestE2E_LoadArchive_RunPod(t *testing.T) {
	k0daBin := getBinaryPath(t)
	rt := findHostContainerRuntime(t)
	name := "k0da-e2e-arch-" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", "")
	image := "k0da-e2e:" + name
	buildTestImage(t, rt, image)
	tar := saveImageToTar(t, rt, image)

	// Ensure cleanup
	t.Cleanup(func() {
		_, _ = runCmd(t, k0daBin, "delete", "--name", name)
	})

	if out, code := runCmd(t, k0daBin, "create", "--name", name, "--timeout", "180s"); code != 0 {
		t.Fatalf("create failed (%d):\n%s", code, out)
	}
	if out, code := runCmd(t, k0daBin, "load", "archive", tar, "-n", name); code != 0 {
		t.Fatalf("load archive failed (%d):\n%s", code, out)
	}

	kubeconfig := getKubeconfigPath(t)
	ctx := "k0da-" + name
	podName := "arch-pod"
	// kubectl run to create a Pod running our image
	if out, code := runKubectl(t, kubeconfig, ctx, "run", podName, "--image="+image, "--restart=Never", "--", "sh", "-c", "sleep 3600"); code != 0 {
		t.Fatalf("kubectl run failed (%d):\n%s", code, out)
	}
	waitForPodReady(t, kubeconfig, ctx, podName, 2*time.Minute)
}

func TestE2E_LoadImage_RunPod(t *testing.T) {
	k0daBin := getBinaryPath(t)
	rt := findHostContainerRuntime(t)
	name := "k0da-e2e-img-" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", "")
	image := "k0da-e2e:" + name
	buildTestImage(t, rt, image)

	// Ensure cleanup
	t.Cleanup(func() {
		_, _ = runCmd(t, k0daBin, "delete", "--name", name)
	})

	if out, code := runCmd(t, k0daBin, "create", "--name", name, "--timeout", "180s"); code != 0 {
		t.Fatalf("create failed (%d):\n%s", code, out)
	}
	if out, code := runCmd(t, k0daBin, "load", "image", image, "-n", name); code != 0 {
		t.Fatalf("load image failed (%d):\n%s", code, out)
	}

	kubeconfig := getKubeconfigPath(t)
	ctx := "k0da-" + name
	podName := "img-pod"
	// kubectl run to create a Pod running our image
	if out, code := runKubectl(t, kubeconfig, ctx, "run", podName, "--image="+image, "--restart=Never", "--", "sh", "-c", "sleep 3600"); code != 0 {
		t.Fatalf("kubectl run failed (%d):\n%s", code, out)
	}
	waitForPodReady(t, kubeconfig, ctx, podName, 2*time.Minute)
}

func TestE2E_Manifests_Mounts_ApplyPod(t *testing.T) {
	bin := buildLocalK0daBinary(t)
	if strings.TrimSpace(bin) != "" {
		// Prefer local binary for this test run
		t.Setenv("K0DA_BIN", bin)
	}
	k0daBin := getBinaryPath(t)
	rt := findHostContainerRuntime(t)
	name := "k0da-e2e-manifest-" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", "")

	// Prepare temp config dir with a pod manifest and cluster config that references it
	cfgDir := t.TempDir()
	podManifest := `apiVersion: v1
kind: Pod
metadata:
  name: manifest-pod
  namespace: default
spec:
  containers:
  - name: pause
    image: registry.k8s.io/pause:3.9
    imagePullPolicy: IfNotPresent
  restartPolicy: Always
`
	if err := os.WriteFile(filepath.Join(cfgDir, "pod.yaml"), []byte(podManifest), 0644); err != nil {
		t.Fatalf("write pod manifest: %v", err)
	}
	cfg := `apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    manifests:
      - ./pod.yaml
  nodes:
    - role: controller
`
	cfgPath := filepath.Join(cfgDir, "cluster.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("write cluster config: %v", err)
	}

	// Ensure cleanup
	t.Cleanup(func() {
		_, _ = runCmd(t, k0daBin, "delete", "--name", name)
	})

	// Create cluster with manifests mounted
	if out, code := runCmd(t, k0daBin, "create", "--name", name, "--timeout", "240s", "-c", cfgPath); code != 0 {
		t.Fatalf("create with manifests failed (%d):\n%s", code, out)
	}

	// Inspect container mounts to ensure the manifest is mounted at the expected target
	mountsOut := inspectContainerMounts(t, rt, name)
	expectedTarget := "/var/lib/k0s/manifests/k0da/000_pod.yaml"
	if !strings.Contains(mountsOut, expectedTarget) {
		t.Fatalf("expected manifest mount target %q not found in mounts:\n%s", expectedTarget, mountsOut)
	}

	// Verify the pod becomes Running
	kubeconfig := getKubeconfigPath(t)
	ctx := "k0da-" + name
	waitForPodReady(t, kubeconfig, ctx, "manifest-pod", 3*time.Minute)
}

func inspectContainerMounts(t *testing.T, runtime string, name string) string {
	t.Helper()
	var out string
	var code int
	switch runtime {
	case "docker":
		out, code = runCmd(t, "docker", "inspect", name, "--format", "{{json .Mounts}}")
	case "podman":
		out, code = runCmd(t, "podman", "inspect", name, "--format", "{{json .Mounts}}")
	default:
		out, code = "", 1
	}
	if code != 0 {
		t.Fatalf("inspect mounts failed (%d): %s", code, out)
	}
	return out
}

func buildLocalK0daBinary(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot get working dir: %v", err)
		return ""
	}
	repoRoot := filepath.Clean(filepath.Join(wd, ".."))
	out := filepath.Join(repoRoot, "build", "k0da-e2e")
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		t.Skipf("cannot create build dir: %v", err)
		return ""
	}
	cmd := exec.Command("go", "build", "-o", out, ".")
	cmd.Dir = repoRoot
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("building k0da failed: %v\n%s", err, string(b))
		return ""
	}
	return out
}
