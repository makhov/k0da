// Package gcr installs gcr (github.com/makhov/gcr), a containerd runtime
// that snapshots, forks, rewinds and migrates running containers, on k0da
// nodes, as runtime class "gcr".
//
// A gcr release is one binary that carries the rest; `gcr setup`, run in a
// node, installs it and registers the runtime with k0s's containerd
// through /etc/k0s/containerd.d, which k0s applies on its own.
package gcr

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/makhov/k0da/internal/runtime"
)

// ReleaseURL is where gcr's releases are downloaded from.
var ReleaseURL = "https://github.com/makhov/gcr/releases/download"

// LatestURL redirects to gcr's latest release, whose tag ends the URL it
// redirects to: no API call, so no rate limit.
var LatestURL = "https://github.com/makhov/gcr/releases/latest"

// Latest is gcr's latest release, such as v0.0.2.
func Latest(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, LatestURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gcr's latest release: %w", err)
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	_, tag, ok := strings.Cut(loc, "/releases/tag/")
	if !ok || !strings.HasPrefix(tag, "v") || strings.Contains(tag, "/") {
		return "", fmt.Errorf("gcr's latest release: %s answered %s, redirecting to %q", LatestURL, resp.Status, loc)
	}
	return tag, nil
}

// Manifests let pods ask for gcr, or, as it is the default, for runc
// instead; and let kubectl gcr migrate and gcr's operator move pods
// between nodes: the gcr-migrate runtime class their targets run on, only
// cluster admins and the operator may use (as in gcr's deploy/gcr.yaml,
// without its node selector: gcr is on every node), and gcr-system, where
// they run their Jobs.
const Manifests = `apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: gcr
handler: gcr
---
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: runc
handler: runc
---
apiVersion: node.k8s.io/v1
kind: RuntimeClass
metadata:
  name: gcr-migrate
handler: gcr-migrate
---
apiVersion: v1
kind: Namespace
metadata:
  name: gcr-system
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: gcr-migrate
spec:
  failurePolicy: Fail
  paramKind:
    apiVersion: v1
    kind: ConfigMap
  matchConstraints:
    resourceRules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["pods"]
  matchConditions:
    - name: on-gcr-migrate
      expression: has(object.spec.runtimeClassName) && object.spec.runtimeClassName == 'gcr-migrate'
  variables:
    - name: annotations
      expression: 'has(object.metadata.annotations) ? object.metadata.annotations : {}'
    - name: restore
      expression: request.namespace + '.' + object.metadata.name
  validations:
    - expression: >-
        'system:masters' in request.userInfo.groups ||
        request.userInfo.username == 'system:serviceaccount:gcr-system:gcr-operator' ||
        (request.userInfo.username in ['system:serviceaccount:kube-system:statefulset-controller', 'system:kube-controller-manager'] &&
        !('gcr.dev/migrate-in' in variables.annotations) &&
        'gcr.dev/restore-token' in variables.annotations && variables.annotations['gcr.dev/restore-token'] != '' &&
        'gcr.dev/fork-from' in variables.annotations &&
        params != null && has(params.data) && variables.restore in params.data &&
        params.data[variables.restore].startsWith('{"token":"' + variables.annotations['gcr.dev/restore-token'] +
        '","dir":"' + variables.annotations['gcr.dev/fork-from'] + '",'))
      message: >-
        only cluster admins and gcr's operator may run pods on runtime class
        gcr-migrate (and StatefulSets, to restore a pod gcr's operator moves):
        its pods make a node's shim listen for a migration, or restore from
        a checkpoint on the node
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: gcr-migrate
spec:
  policyName: gcr-migrate
  paramRef:
    name: gcr-restores
    namespace: gcr-system
    # This file makes it: were it missing, no pod could run on
    # gcr-migrate, rather than any.
    parameterNotFoundAction: Deny
  validationActions: [Deny]
---
# The pending restores of StatefulSets' pods gcr's operator moves:
# NAMESPACE.NAME to the restore, which it writes, and the gcr-migrate
# policy reads.
apiVersion: v1
kind: ConfigMap
metadata:
  name: gcr-restores
  namespace: gcr-system
`

// defaultRuntime makes gcr the runtime of pods that name none, beside the
// drop-in gcr setup writes.
const defaultRuntime = `# Written by k0da: pods without a runtimeClassName run on gcr.
version = 3
[plugins.'io.containerd.cri.v1.runtime'.containerd]
  default_runtime_name = 'gcr'
`

// defaultRuntime17 is defaultRuntime for containerd 1.7 (k0s v1.33 and
// earlier), which reads version 2 drop-ins only.
const defaultRuntime17 = `# Written by k0da: pods without a runtimeClassName run on gcr.
version = 2
[plugins."io.containerd.grpc.v1.cri".containerd]
  default_runtime_name = "gcr"
`

// inNode is where gcr goes in a node, and the shim with it: a directory
// of its own, as libcontainer seals its binary by overlay-mounting the
// binary's directory.
const inNode = "/opt/gcr"

// Binary returns the path of the gcr binary for arch: binary itself if
// given, else release version downloaded into cache (once) and checked
// against the release's SHA256SUMS.
func Binary(ctx context.Context, version, binary, arch, cache string) (string, error) {
	if binary != "" {
		if _, err := os.Stat(binary); err != nil {
			return "", fmt.Errorf("gcr binary: %w", err)
		}
		return binary, nil
	}
	name := fmt.Sprintf("gcr-%s-linux-%s", version, arch)
	path := filepath.Join(cache, name)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return "", err
	}
	sums, err := fetch(ctx, version, "SHA256SUMS")
	if err != nil {
		return "", err
	}
	want := ""
	s := bufio.NewScanner(strings.NewReader(string(sums)))
	for s.Scan() {
		if f := strings.Fields(s.Text()); len(f) == 2 && f[1] == name {
			want = f[0]
		}
	}
	if want == "" {
		return "", fmt.Errorf("gcr %s has no %s (SHA256SUMS)", version, name)
	}
	b, err := fetch(ctx, version, name)
	if err != nil {
		return "", err
	}
	if got := sha256.Sum256(b); hex.EncodeToString(got[:]) != want {
		return "", fmt.Errorf("gcr %s: %s does not match its checksum", version, name)
	}
	tmp := path + ".download"
	if err := os.WriteFile(tmp, b, 0o755); err != nil {
		return "", err
	}
	return path, os.Rename(tmp, path)
}

func fetch(ctx context.Context, version, name string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	u := fmt.Sprintf("%s/%s/%s", ReleaseURL, version, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", u, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", u, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// NodeArch is the architecture of node's containers, as Go names it.
func NodeArch(ctx context.Context, b runtime.Runtime, node string) (string, error) {
	out, code, err := b.ExecInContainer(ctx, node, []string{"uname", "-m"})
	if err != nil || code != 0 {
		return "", fmt.Errorf("uname in %s: %v (exit %d)", node, err, code)
	}
	switch m := strings.TrimSpace(out); m {
	case "x86_64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("gcr has no release for %s", m)
	}
}

// WaitForK0s waits until k0s has written its containerd configuration in
// node, which gcr setup adds gcr to. A worker writes it once it has joined;
// until its container runs, commands in it fail, which counts as not yet.
func WaitForK0s(ctx context.Context, b runtime.Runtime, node string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, code, err := b.ExecInContainer(ctx, node, []string{"test", "-f", "/etc/k0s/containerd.toml"}); err == nil && code == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("k0s did not write its containerd configuration in %s within %s (has the node joined?)", node, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// Install copies gcr into node and runs its setup there (WaitForK0s
// first).
func Install(ctx context.Context, b runtime.Runtime, node, binary string) error {
	if _, code, err := b.ExecInContainer(ctx, node, []string{"mkdir", "-p", inNode}); err != nil || code != 0 {
		return fmt.Errorf("mkdir %s in %s: %v (exit %d)", inNode, node, err, code)
	}
	if err := b.CopyToContainer(ctx, node, binary, inNode+"/gcr"); err != nil {
		return fmt.Errorf("copy gcr into %s: %w", node, err)
	}
	// The binaries go in the node's PATH: gcr's doctor, run by setup, looks
	// for the shim there (before v0.0.2).
	out, code, err := b.ExecInContainer(ctx, node, []string{inNode + "/gcr", "setup", "-bin-dir", "/usr/local/bin"})
	if err != nil || code != 0 {
		return fmt.Errorf("gcr setup in %s (exit %d): %v\n%s", node, code, err, out)
	}
	// After gcr's own drop-in: a default naming a runtime containerd does
	// not have yet would stop its CRI plugin.
	tmp, err := os.CreateTemp("", "gcr-default-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	dropIn := defaultRuntime
	if out, _, err := b.ExecInContainer(ctx, node, []string{"/var/lib/k0s/bin/containerd", "--version"}); err == nil && containerd17(out) {
		dropIn = defaultRuntime17
	}
	if _, err := tmp.WriteString(dropIn); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()
	if err := b.CopyToContainer(ctx, node, tmp.Name(), "/etc/k0s/containerd.d/gcr-default.toml"); err != nil {
		return fmt.Errorf("make gcr the default runtime in %s: %w", node, err)
	}
	return nil
}

// containerd17 says whether containerd --version printed a 1.x release:
// "containerd github.com/containerd/containerd 1.7.27 …" from k0s's builds,
// "v1.7.29" from containerd's.
func containerd17(version string) bool {
	f := strings.Fields(version)
	return len(f) >= 3 && strings.HasPrefix(strings.TrimPrefix(f[2], "v"), "1.")
}
