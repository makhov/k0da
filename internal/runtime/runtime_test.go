package runtime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMounts_ToBinds(t *testing.T) {
	ms := Mounts{
		{Type: "bind", Source: "/host/a", Target: "/ctr/a", Options: []string{"ro"}},
		{Type: "volume", Source: "named-vol", Target: "/data"},
		{Type: "tmpfs", Source: "ignored", Target: "/tmp"},
	}
	b := ms.ToBinds()
	require.Len(t, b, 2)
	require.Equal(t, "/host/a:/ctr/a:ro", b[0])
	require.Equal(t, "named-vol:/data", b[1])
}

func TestEnvVars(t *testing.T) {
	ev := EnvVars{{Name: "A", Value: "1"}, {Name: "B", Value: "2"}}
	arr := ev.ToOSStrings()
	require.Equal(t, []string{"A=1", "B=2"}, arr)
	m := ev.ToMap()
	require.Equal(t, map[string]string{"A": "1", "B": "2"}, m)
}
