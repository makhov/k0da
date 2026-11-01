package gcr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLatestChart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/token":
			_, _ = w.Write([]byte(`{"token":"t0k"}`))
		case r.URL.Path == "/v2/makhov/charts/gcr/tags/list" && r.Header.Get("Authorization") == "Bearer t0k":
			_, _ = w.Write([]byte(`{"tags":["0.1.0","0.10.0","0.2.0","0.11.0-rc.1","latest","0.9.9"]}`))
		default:
			http.Error(w, "no", http.StatusUnauthorized)
		}
	}))
	defer srv.Close()
	old := RegistryURL
	RegistryURL = srv.URL
	defer func() { RegistryURL = old }()
	got, err := LatestChart(context.Background())
	if err != nil || got != "0.10.0" {
		t.Fatalf("got %q, %v; want 0.10.0", got, err)
	}
}

func TestHelmChart(t *testing.T) {
	entry, err := HelmChart("0.1.1", "v0.1.6", true, map[string]any{
		"operator": map[string]any{"registry": "reg.example/gcr"},
		"runtime":  map[string]any{"installer": map[string]any{"setupArgs": "-x"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry["chartname"] != Chart || entry["version"] != "0.1.1" || entry["namespace"] != "gcr-system" {
		t.Fatalf("entry %v", entry)
	}
	var v map[string]any
	if err := yaml.Unmarshal([]byte(entry["values"].(string)), &v); err != nil {
		t.Fatal(err)
	}
	get := func(path string) any {
		var cur any = v
		for _, k := range strings.Split(path, ".") {
			cur = cur.(map[string]any)[k]
		}
		return cur
	}
	for path, want := range map[string]any{
		"runtime.installer.enabled":   false, // k0da installs gcr
		"runtime.installer.setupArgs": "-x",  // a value given, merged
		"runtime.readyNodesOnly":      false, // gcr is on every node
		"operator.enabled":            true,  // asked for
		"operator.registry":           "reg.example/gcr",
		"image.tag":                   "v0.1.6", // the release on the nodes
	} {
		if got := get(path); got != want {
			t.Errorf("%s = %v, want %v", path, got, want)
		}
	}
	// A binary's: the chart's own image.
	entry, _ = HelmChart("0.1.1", "", false, nil)
	if strings.Contains(entry["values"].(string), "image") {
		t.Errorf("values with no image: %s", entry["values"])
	}
}
