package runtime

import "testing"

func TestNormalizeSocket(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"unix:///var/run/docker.sock", "unix:///var/run/docker.sock"},
		{"/var/run/docker.sock", "unix:///var/run/docker.sock"},
		{"unix:/var/run/docker.sock", "unix:///var/run/docker.sock"},
		{"ssh://root@host/run/podman/podman.sock", "ssh://root@host/run/podman/podman.sock"},
	}
	for _, c := range cases {
		if got := normalizeSocket(c.in); got != c.want {
			t.Fatalf("normalizeSocket(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
