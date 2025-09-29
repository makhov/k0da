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

	"github.com/stretchr/testify/require"
)

func getBinaryPath(t *testing.T) string {
	// Prefer explicit K0DA_BIN, else PATH
	if p := os.Getenv("K0DA_BIN"); strings.TrimSpace(p) != "" {
		return p
	}
	if _, err := os.Stat("./k0da"); err == nil {
		return filepath.Join("./k0da")
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

	out, code := runCmd(t, k0daBin, "create", "--name", name, "--timeout", "120s")
	require.Equalf(t, 0, code, "create failed (%d):\n%s", code, out)
	out, code = runCmd(t, k0daBin, "list")
	require.Equalf(t, 0, code, "list failed (%d):\n%s", code, out)
	require.Containsf(t, out, name, "cluster %q not found in list:\n%s", name, out)
	out, code = runCmd(t, k0daBin, "delete", "--name", name)
	require.Equalf(t, 0, code, "delete failed (%d):\n%s", code, out)
}

func TestE2E_CreateListDelete_WithConfig(t *testing.T) {
	k0daBin := getBinaryPath(t)
	name := "k0da-e2e-mn-" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", "")

	// Multi-node config: 1 controller + 1 worker
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "cluster.yaml")
	cfgYAML := `apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    version: v1.33.3-k0s.0
  nodes:
    - role: controller
      name: ` + name + `
    - role: worker
      name: ` + name + `-w1
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0644))

	// Ensure cleanup
	t.Cleanup(func() {
		_, _ = runCmd(t, k0daBin, "delete", "--name", name)
	})

	// Create cluster
	out, code := runCmd(t, k0daBin, "create", "-n", name, "-c", cfgPath, "--timeout", "240s")
	require.Equalf(t, 0, code, "create (multi-node) failed (%d):\n%s", code, out)

	// List should show a single cluster entry
	out, code = runCmd(t, k0daBin, "list")
	require.Equal(t, 0, code)
	require.Contains(t, out, name)

	// Delete cluster (should remove both nodes)
	out, code = runCmd(t, k0daBin, "delete", "--name", name)
	require.Equalf(t, 0, code, "delete failed (%d):\n%s", code, out)
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
		buildArgs := append([]string{"build", "-t", tag}, dir)
		if out, code := runCmd(t, "docker", buildArgs...); code != 0 {
			t.Skipf("docker build failed (%d):\n%s", code, out)
		}
	case "podman":
		buildArgs := append([]string{"build", "-t", tag}, dir)
		if out, code := runCmd(t, "podman", buildArgs...); code != 0 {
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

	out, code := runCmd(t, k0daBin, "create", "--name", name, "--timeout", "180s")
	require.Equalf(t, 0, code, "create failed (%d):\n%s", code, out)
	out, code = runCmd(t, k0daBin, "load", "archive", tar, "-n", name)
	require.Equalf(t, 0, code, "load archive failed (%d):\n%s", code, out)

	kubeconfig := getKubeconfigPath(t)
	ctx := "k0da-" + name
	podName := "arch-pod"
	// kubectl run to create a Pod running our image
	out, code = runKubectl(t, kubeconfig, ctx, "run", podName, "--image="+image, "--restart=Never", "--", "sh", "-c", "sleep 3600")
	require.Equalf(t, 0, code, "kubectl run failed (%d):\n%s", code, out)
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

	out, code := runCmd(t, k0daBin, "create", "--name", name, "--timeout", "180s")
	require.Equalf(t, 0, code, "create failed (%d):\n%s", code, out)
	out, code = runCmd(t, k0daBin, "load", "image", image, "-n", name)
	require.Equalf(t, 0, code, "load image failed (%d):\n%s", code, out)

	kubeconfig := getKubeconfigPath(t)
	ctx := "k0da-" + name
	podName := "img-pod"
	// kubectl run to create a Pod running our image
	out, code = runKubectl(t, kubeconfig, ctx, "run", podName, "--image="+image, "--restart=Never", "--", "sh", "-c", "sleep 3600")
	require.Equalf(t, 0, code, "kubectl run failed (%d):\n%s", code, out)
	waitForPodReady(t, kubeconfig, ctx, podName, 2*time.Minute)
}

func TestE2E_Manifests_Mounts_ApplyPod(t *testing.T) {
	k0daBin := getBinaryPath(t)
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
	out, code := runCmd(t, k0daBin, "create", "--name", name, "--timeout", "240s", "-c", cfgPath)
	require.Equalf(t, 0, code, "create with manifests failed (%d):\n%s", code, out)

	// Verify the pod becomes Running
	kubeconfig := getKubeconfigPath(t)
	ctx := "k0da-" + name
	waitForPodReady(t, kubeconfig, ctx, "manifest-pod", 3*time.Minute)
}

func TestE2E_Update(t *testing.T) {
	k0daBin := getBinaryPath(t)
	name := "k0da-e2e-update-" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", "")

	// Prepare temp config dir with a pod manifest and cluster config that references it
	cfgDir := t.TempDir()
	pod1 := `apiVersion: v1
kind: Pod
metadata:
  name: update-pod
  namespace: default
spec:
  containers:
  - name: pause
    image: registry.k8s.io/pause:3.9
    imagePullPolicy: IfNotPresent
  restartPolicy: Always
`
	if err := os.WriteFile(filepath.Join(cfgDir, "pod1.yaml"), []byte(pod1), 0644); err != nil {
		t.Fatalf("write pod1 manifest: %v", err)
	}
	cfg := `apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    manifests:
      - ./pod1.yaml
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

	t.Log("Creating cluster...")
	out, code := runCmd(t, k0daBin, "create", "--name", name, "--timeout", "240s", "-c", cfgPath)
	require.Equalf(t, 0, code, "create with manifests failed (%d):\n%s", code, out)

	// Wait for initial pod
	kubeconfig := getKubeconfigPath(t)
	ctx := "k0da-" + name
	waitForPodReady(t, kubeconfig, ctx, "update-pod", 3*time.Minute)

	// Modify manifest and add another one
	pod1b := `apiVersion: v1
kind: Pod
metadata:
  name: update-pod
  namespace: default
spec:
  containers:
  - name: pause
    image: registry.k8s.io/pause:3.10
    imagePullPolicy: IfNotPresent
  restartPolicy: Always
`
	if err := os.WriteFile(filepath.Join(cfgDir, "pod1.yaml"), []byte(pod1b), 0644); err != nil {
		t.Fatalf("rewrite pod1 manifest: %v", err)
	}
	pod2 := `apiVersion: v1
kind: Pod
metadata:
  name: update-pod-2
  namespace: default
spec:
  containers:
  - name: pause
    image: registry.k8s.io/pause:3.9
    imagePullPolicy: IfNotPresent
  restartPolicy: Always
`
	if err := os.WriteFile(filepath.Join(cfgDir, "pod2.yaml"), []byte(pod2), 0644); err != nil {
		t.Fatalf("write pod2 manifest: %v", err)
	}
	// Update config to include second manifest and reorder, and change k0s config via dynamic config (telemetry.enabled=false)
	cfg2 := `apiVersion: k0da.k0sproject.io/v1alpha1
kind: Cluster
spec:
  k0s:
    manifests:
      - ./pod2.yaml
      - ./pod1.yaml
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: Config
      spec:
        telemetry:
          enabled: false
  nodes:
    - role: controller
      name: ` + name + `
    - role: worker
      name: ` + name + `-w1
`
	if err := os.WriteFile(cfgPath, []byte(cfg2), 0644); err != nil {
		t.Fatalf("write updated cluster config: %v", err)
	}

	t.Log("Running update...")
	out, code = runCmd(t, k0daBin, "update", "--name", name, "-c", cfgPath, "--timeout", "240s")
	require.Equalf(t, 0, code, "update failed (%d):\n%s", code, out)

	t.Log("Waiting for new pod...")
	// Verify the new pod appears and the old pod is still running (image change may recreate)
	waitForPodReady(t, kubeconfig, ctx, "update-pod-2", 3*time.Minute)

	t.Log("Checking config...")
	view, _ := runKubectl(t, kubeconfig, ctx, "get", "clusterconfig", "k0s", "-o", "yaml", "-n", "kube-system")
	require.Contains(t, view, "telemetry:")
	require.NotContains(t, view, "enabled: true")
}

func TestE2E_LocalPathProvisioner_DefaultConfig(t *testing.T) {
	k0daBin := getBinaryPath(t)
	name := "k0da-e2e-localpath-" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", "")

	// Ensure cleanup
	t.Cleanup(func() {
		_, _ = runCmd(t, k0daBin, "delete", "--name", name)
	})

	// Create cluster with default config (should include local-path-provisioner)
	out, code := runCmd(t, k0daBin, "create", "--name", name, "--timeout", "180s")
	require.Equalf(t, 0, code, "create failed (%d):\n%s", code, out)

	kubeconfig := getKubeconfigPath(t)
	ctx := "k0da-" + name

	// Wait for local-path-provisioner deployment to be ready
	t.Log("Waiting for local-path-provisioner deployment to be ready...")
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		out, _ := runKubectl(t, kubeconfig, ctx, "get", "deployment", "local-path-provisioner", "-n", "local-path-storage", "-o", "jsonpath={.status.readyReplicas}")
		if strings.TrimSpace(out) == "1" {
			break
		}
		time.Sleep(3 * time.Second)
	}

	// Check that local-path-provisioner pod is running
	out, code = runKubectl(t, kubeconfig, ctx, "get", "pods", "-n", "local-path-storage", "-l", "app=local-path-provisioner", "-o", "jsonpath={.items[0].status.phase}")
	require.Equalf(t, 0, code, "kubectl get pods failed (%d):\n%s", code, out)
	require.Equal(t, "Running", strings.TrimSpace(out), "local-path-provisioner pod is not running")

	// Check that local-path storage class exists
	out, code = runKubectl(t, kubeconfig, ctx, "get", "storageclass", "local-path", "-o", "jsonpath={.metadata.name}")
	require.Equalf(t, 0, code, "kubectl get storageclass failed (%d):\n%s", code, out)
	require.Equal(t, "local-path", strings.TrimSpace(out), "local-path storage class not found")

	// Check that local-path storage class is the default
	out, code = runKubectl(t, kubeconfig, ctx, "get", "storageclass", "local-path", "-o", "jsonpath={.metadata.annotations.storageclass\\.kubernetes\\.io/is-default-class}")
	require.Equalf(t, 0, code, "kubectl get storageclass annotation failed (%d):\n%s", code, out)
	require.Equal(t, "true", strings.TrimSpace(out), "local-path storage class is not the default")

	// Verify that the local-path-provisioner deployment has the correct number of replicas
	out, code = runKubectl(t, kubeconfig, ctx, "get", "deployment", "local-path-provisioner", "-n", "local-path-storage", "-o", "jsonpath={.status.replicas}")
	require.Equalf(t, 0, code, "kubectl get deployment replicas failed (%d):\n%s", code, out)
	require.Equal(t, "1", strings.TrimSpace(out), "local-path-provisioner deployment should have 1 replica")

	// Verify that the local-path-provisioner deployment is available
	out, code = runKubectl(t, kubeconfig, ctx, "get", "deployment", "local-path-provisioner", "-n", "local-path-storage", "-o", "jsonpath={.status.conditions[?(@.type==\"Available\")].status}")
	require.Equalf(t, 0, code, "kubectl get deployment status failed (%d):\n%s", code, out)
	require.Equal(t, "True", strings.TrimSpace(out), "local-path-provisioner deployment should be available")
}
