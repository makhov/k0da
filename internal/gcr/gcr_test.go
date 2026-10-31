package gcr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinary(t *testing.T) {
	body := []byte("a gcr binary")
	sum := sha256.Sum256(body)
	sums := hex.EncodeToString(sum[:]) + "  gcr-v0.0.1-linux-arm64\n"
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/v0.0.1/SHA256SUMS":
			_, _ = w.Write([]byte(sums))
		case "/v0.0.1/gcr-v0.0.1-linux-arm64":
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ReleaseURL = srv.URL
	cache := t.TempDir()

	path, err := Binary(context.Background(), "v0.0.1", "", "arm64", cache)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != string(body) {
		t.Fatalf("downloaded %q", b)
	}
	// Cached: no second download.
	n := requests
	if _, err := Binary(context.Background(), "v0.0.1", "", "arm64", cache); err != nil || requests != n {
		t.Fatalf("second call: %v, %d requests", err, requests-n)
	}
	// No such architecture in the release.
	if _, err := Binary(context.Background(), "v0.0.1", "", "amd64", cache); err == nil || !strings.Contains(err.Error(), "has no") {
		t.Fatalf("missing architecture: %v", err)
	}
	// A binary that does not match its checksum is refused.
	sums = strings.Repeat("0", 64) + "  gcr-v0.0.1-linux-arm64\n"
	if _, err := Binary(context.Background(), "v0.0.1", "", "arm64", t.TempDir()); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("bad checksum: %v", err)
	}
	// A given binary is used as it is.
	local := filepath.Join(t.TempDir(), "gcr")
	if err := os.WriteFile(local, body, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := Binary(context.Background(), "", local, "arm64", cache); err != nil || got != local {
		t.Fatalf("local binary: %q, %v", got, err)
	}
}

func TestContainerd17(t *testing.T) {
	for out, want := range map[string]bool{
		"containerd github.com/containerd/containerd 1.7.27 05044ec0a9a75232cad458027ca83437aae3f4da\n": true,
		"containerd github.com/containerd/containerd v1.7.29 442cb34b\n":                                true,
		"containerd github.com/containerd/containerd/v2 v2.2.1 cf6bb5a7\n":                              false,
		"": false,
	} {
		if got := containerd17(out); got != want {
			t.Errorf("%q: %v, want %v", out, got, want)
		}
	}
}
