package gcr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Chart is gcr's Helm chart, which installs gcr's runtime classes (gcr,
// and gcr-migrate for migrations' targets), the admission policy that
// guards gcr-migrate, and, optionally, gcr-operator. k0s installs it
// (extensions.helm), with its installer off: k0da puts gcr on the nodes.
const Chart = "oci://ghcr.io/makhov/charts/gcr"

// ChartNamespace is where the chart goes: gcr-system, where kubectl gcr and
// the operator run their Jobs.
const ChartNamespace = "gcr-system"

// RegistryURL is the registry the chart is in.
var RegistryURL = "https://ghcr.io"

// chartRepository is Chart's repository in RegistryURL.
const chartRepository = "makhov/charts/gcr"

// LatestChart is the latest version of gcr's chart, such as 0.1.1: its
// tags in the registry, read anonymously.
func LatestChart(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var token struct {
		Token string `json:"token"`
	}
	if err := getJSON(ctx, RegistryURL+"/token?scope=repository:"+chartRepository+":pull", "", &token); err != nil {
		return "", fmt.Errorf("gcr's chart: %w", err)
	}
	var tags struct {
		Tags []string `json:"tags"`
	}
	if err := getJSON(ctx, RegistryURL+"/v2/"+chartRepository+"/tags/list", token.Token, &tags); err != nil {
		return "", fmt.Errorf("gcr's chart: %w", err)
	}
	latest, best := "", [3]int{-1}
	for _, t := range tags.Tags {
		if v, ok := release(t); ok && newer(v, best) {
			latest, best = t, v
		}
	}
	if latest == "" {
		return "", fmt.Errorf("gcr's chart: no release among %s's tags %v", Chart, tags.Tags)
	}
	return latest, nil
}

func getJSON(ctx context.Context, url, token string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

// release parses a release version, X.Y.Z, with nothing after it.
func release(t string) ([3]int, bool) {
	var v [3]int
	parts := strings.Split(t, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func newer(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// HelmChart is the k0s Helm extension's entry for gcr's chart, at version:
// its installer off, its runtime classes on every node (gcr is), and the
// operator if asked for, running image (a gcr release, such as v0.1.6;
// "" is the chart's own). values are more of the chart's values, over
// these.
func HelmChart(version, image string, operator bool, values map[string]any) (map[string]any, error) {
	v := map[string]any{
		"runtime": map[string]any{
			"installer":      map[string]any{"enabled": false},
			"readyNodesOnly": false,
		},
		"operator": map[string]any{"enabled": operator},
	}
	if image != "" {
		v["image"] = map[string]any{"tag": image}
	}
	merge(v, values)
	out, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"name":      "gcr",
		"chartname": Chart,
		"version":   version,
		"namespace": ChartNamespace,
		"values":    string(out),
	}, nil
}

// merge sets src's values in dst, merging maps in both, as Helm merges
// values files.
func merge(dst, src map[string]any) {
	for k, sv := range src {
		if sm, ok := sv.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				merge(dm, sm)
				continue
			}
		}
		dst[k] = sv
	}
}
