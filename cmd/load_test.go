package cmd

import (
	"context"
	"reflect"
	"strings"
	"testing"

	k0daconfig "github.com/makhov/k0da/internal/config"
	"github.com/makhov/k0da/internal/runtime"
)

// loadRuntime records what loading does; the rest of runtime.Runtime is
// not called.
type loadRuntime struct {
	runtime.Runtime
	nodes    []runtime.ContainerInfo
	selector map[string]string
	copied   []string
	imported []string
}

func (r *loadRuntime) ListContainersByLabel(_ context.Context, sel map[string]string, _ bool) ([]runtime.ContainerInfo, error) {
	r.selector = sel
	return r.nodes, nil
}

func (r *loadRuntime) CopyToContainer(_ context.Context, name, _, dst string) error {
	r.copied = append(r.copied, name+":"+dst)
	return nil
}

func (r *loadRuntime) ExecInContainer(_ context.Context, name string, cmd []string) (string, int, error) {
	if cmd[0] == "k0s" {
		r.imported = append(r.imported, name+":"+strings.Join(cmd[1:], " "))
	}
	return "", 0, nil
}

func TestImportToNodes(t *testing.T) {
	r := &loadRuntime{nodes: []runtime.ContainerInfo{{Name: "c"}, {Name: "c-worker-0"}, {Name: "c-worker-1"}}}
	if err := importToNodes(context.Background(), r, "c", "/x/images.tar"); err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{k0daconfig.LabelClusterName: "c"}; !reflect.DeepEqual(r.selector, want) {
		t.Errorf("nodes listed by %v, want %v", r.selector, want)
	}
	if want := []string{"c:/tmp/images.tar", "c-worker-0:/tmp/images.tar", "c-worker-1:/tmp/images.tar"}; !reflect.DeepEqual(r.copied, want) {
		t.Errorf("copied to %v, want %v", r.copied, want)
	}
	if len(r.imported) != 3 || !strings.HasPrefix(r.imported[2], "c-worker-1:ctr -n k8s.io images import /tmp/images.tar") {
		t.Errorf("imported %v, want one import per node", r.imported)
	}
}

func TestImportToNodesNone(t *testing.T) {
	if err := importToNodes(context.Background(), &loadRuntime{}, "c", "/x/images.tar"); err == nil {
		t.Error("loaded into a cluster with no nodes")
	}
}
