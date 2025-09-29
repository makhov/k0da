package config

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// StableVersionURL is the source of truth for the latest stable k0s version.
const StableVersionURL = "https://docs.k0sproject.io/stable.txt"

// FetchStableK0sVersion retrieves the latest stable k0s version string.
// It returns values like "v1.33.4+k0s.0" as published by k0s docs.
func FetchStableK0sVersion(client *http.Client) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(StableVersionURL)
	if err != nil {
		return "", fmt.Errorf("fetch stable version: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch stable version: unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read stable version: %w", err)
	}
	ver := strings.TrimSpace(string(data))
	return ver, nil
}

// StableVersionAsImageTag converts the stable version published as
// "vX.Y.Z+k0s.N" into the image tag format "vX.Y.Z-k0s.N" used by k0s images.
func StableVersionAsImageTag(stable string) string {
	return NormalizeVersionTag(stable)
}

// IsNewerStableThanDefault compares the fetched stable version against
// the compiled default and returns true if the remote stable is newer.
// With normalization applied and magic removed, any normalized difference
// indicates a newer remote version.
func IsNewerStableThanDefault(stable string) bool {
	stable = NormalizeVersionTag(stable)
	if stable == "" {
		return false
	}
	if stable == NormalizeVersionTag(DefaultK0sVersion) {
		return false
	}
	return true
}
