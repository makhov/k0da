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
