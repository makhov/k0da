package runtime

import "testing"

func TestMounts_ToBinds(t *testing.T) {
	ms := Mounts{
		{Type: "bind", Source: "/host/a", Target: "/ctr/a", Options: []string{"ro"}},
		{Type: "volume", Source: "named-vol", Target: "/data"},
		{Type: "tmpfs", Source: "ignored", Target: "/tmp"},
	}
	b := ms.ToBinds()
	if len(b) != 2 {
		t.Fatalf("expected 2 bind entries, got %d", len(b))
	}
	if b[0] != "/host/a:/ctr/a:ro" {
		t.Fatalf("unexpected binds[0]: %q", b[0])
	}
	if b[1] != "named-vol:/data" {
		t.Fatalf("unexpected binds[1]: %q", b[1])
	}
}

func TestEnvVars(t *testing.T) {
	ev := EnvVars{{Name: "A", Value: "1"}, {Name: "B", Value: "2"}}
	arr := ev.ToOSStrings()
	if len(arr) != 2 || arr[0] != "A=1" || arr[1] != "B=2" {
		t.Fatalf("unexpected ToOSStrings: %v", arr)
	}
	m := ev.ToMap()
	if len(m) != 2 || m["A"] != "1" || m["B"] != "2" {
		t.Fatalf("unexpected ToMap: %v", m)
	}
}
