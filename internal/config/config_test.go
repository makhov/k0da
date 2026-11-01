package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEffectiveK0sConfig_Default(t *testing.T) {
	var cc *ClusterConfig
	// nil receiver usage guarded; construct empty config to call method
	empty := &ClusterConfig{}
	cfg := empty.EffectiveK0sConfig()

	require.Equal(t, "k0s.k0sproject.io/v1beta1", cfg["apiVersion"])
	require.Equal(t, "ClusterConfig", cfg["kind"])

	spec, ok := cfg["spec"].(map[string]any)
	require.True(t, ok)
	require.NotNil(t, spec)
	require.GreaterOrEqual(t, len(spec), 0)

	// Ensure cc remains unused to avoid lint warnings
	_ = cc
}

func TestEffectiveK0sConfig_MergesNestedMaps(t *testing.T) {
	cc := &ClusterConfig{}
	cc.Spec.K0s.Config = map[string]any{
		"spec": map[string]any{
			"telemetry": map[string]any{
				"enabled": false,
			},
		},
	}

	cfg := cc.EffectiveK0sConfig()
	require.Equal(t, "k0s.k0sproject.io/v1beta1", cfg["apiVersion"])
	require.Equal(t, "ClusterConfig", cfg["kind"])

	spec, ok := cfg["spec"].(map[string]any)
	require.True(t, ok)
	tel, ok := spec["telemetry"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, tel["enabled"])
}

func TestEffectiveK0sConfig_SuboptimalUserConfig(t *testing.T) {
	cc := &ClusterConfig{}
	cc.Spec.K0s.Config = map[string]any{
		"apiVersion": "k0s.k0sproject.io/v1beta1",
		"kind":       "Config",
		"metadata":   map[string]string{"name": "custom-name", "namespace": "custom-namespace"},
		"spec": map[string]any{
			"telemetry": map[string]any{
				"enabled": false,
			},
		},
	}

	cfg := cc.EffectiveK0sConfig()
	require.Equal(t, "k0s.k0sproject.io/v1beta1", cfg["apiVersion"])
	require.Equal(t, "ClusterConfig", cfg["kind"])

	metadata, ok := cfg["metadata"].(map[string]string)
	require.True(t, ok)
	require.Equal(t, "k0s", metadata["name"])
	require.Equal(t, "kube-system", metadata["namespace"])
}

func TestEffectiveK0sConfig_OverrideDoesNotRemoveDefaultKeys(t *testing.T) {
	cc := &ClusterConfig{}
	// Provide an unrelated override under spec to ensure base keys still present
	cc.Spec.K0s.Config = map[string]any{
		"spec": map[string]any{
			"someFeature": map[string]any{
				"flag": true,
			},
		},
	}

	cfg := cc.EffectiveK0sConfig()

	// Default top-level keys remain
	require.Equal(t, "k0s.k0sproject.io/v1beta1", cfg["apiVersion"])
	require.Equal(t, "ClusterConfig", cfg["kind"])

	spec, ok := cfg["spec"].(map[string]any)
	require.True(t, ok)
	feat, ok := spec["someFeature"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, feat["flag"])
}

func TestGcrSpec(t *testing.T) {
	for name, tc := range map[string]struct {
		spec    *GcrSpec
		source  string
		wantErr bool
		want    string
	}{
		"version":         {spec: &GcrSpec{Version: "v0.0.1"}},
		"binary relative": {spec: &GcrSpec{Binary: "bin/gcr"}, source: "/x/cluster.yaml", want: "/x/bin/gcr"},
		"binary absolute": {spec: &GcrSpec{Binary: "/opt/gcr"}, source: "/x/cluster.yaml", want: "/opt/gcr"},
		"both":            {spec: &GcrSpec{Version: "v0.0.1", Binary: "gcr"}, wantErr: true},
		"neither: latest": {spec: &GcrSpec{}},
		"latest":          {spec: &GcrSpec{Version: "latest"}},
	} {
		t.Run(name, func(t *testing.T) {
			c := ClusterConfig{Spec: Spec{Gcr: tc.spec}, SourcePath: tc.source}
			err := c.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("error: %v", err)
			}
			if tc.want != "" && c.Spec.Gcr.Binary != tc.want {
				t.Fatalf("binary %q, want %q", c.Spec.Gcr.Binary, tc.want)
			}
		})
	}
}

// gcr's chart goes after the charts the cluster config has, into the
// effective k0s config.
func TestAddHelmChart(t *testing.T) {
	c := &ClusterConfig{}
	c.Spec.K0s.Config = map[string]any{"spec": map[string]any{
		"extensions": map[string]any{"helm": map[string]any{"charts": []any{map[string]any{"name": "theirs"}}}},
		"network":    map[string]any{"provider": "calico"},
	}}
	c.AddHelmChart(map[string]any{"name": "gcr"})
	eff := c.EffectiveK0sConfig()
	spec := eff["spec"].(map[string]any)
	charts := spec["extensions"].(map[string]any)["helm"].(map[string]any)["charts"].([]any)
	if len(charts) != 2 || charts[0].(map[string]any)["name"] != "theirs" || charts[1].(map[string]any)["name"] != "gcr" {
		t.Fatalf("charts %v", charts)
	}
	if spec["network"].(map[string]any)["provider"] != "calico" {
		t.Errorf("lost the rest of the config: %v", spec)
	}
	// And with no k0s config at all.
	c = &ClusterConfig{}
	c.AddHelmChart(map[string]any{"name": "gcr"})
	if n := len(c.EffectiveK0sConfig()["spec"].(map[string]any)["extensions"].(map[string]any)["helm"].(map[string]any)["charts"].([]any)); n != 1 {
		t.Fatalf("%d charts", n)
	}
}
