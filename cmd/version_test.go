package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	Version = "v1.2.3"
	Commit = "abc1234"
	BuildDate = "2025-01-01T00:00:00Z"

	buf := new(bytes.Buffer)
	versionCmd.SetOut(buf)
	versionCmd.SetErr(buf)
	// Call the Run function directly to avoid root command parsing
	versionCmd.Run(versionCmd, []string{})
	out := buf.String()
	if !strings.Contains(out, "k0da v1.2.3") || !strings.Contains(out, "abc1234") {
		t.Fatalf("unexpected output: %q", out)
	}
}
