package cmd

import (
	"fmt"
	"net/http"
	"time"

	k0daconfig "github.com/makhov/k0da/internal/config"
	"github.com/spf13/cobra"
)

var (
	Version   = "dev"
	Commit    = ""
	BuildDate = ""
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "k0da %s", Version)
		if Commit != "" {
			_, _ = fmt.Fprintf(w, " (commit %s)", Commit)
		}
		if BuildDate != "" {
			_, _ = fmt.Fprintf(w, " built %s", BuildDate)
		}
		_, _ = fmt.Fprintln(w)

		if check, _ := cmd.Flags().GetBool("check-latest"); check {
			client := &http.Client{Timeout: 3 * time.Second}
			if stable, err := k0daconfig.FetchStableK0sVersion(client); err == nil {
				stableTag := k0daconfig.StableVersionAsImageTag(stable)
				current := k0daconfig.NormalizeVersionTag(k0daconfig.DefaultK0sVersion)
				if stableTag != current {
					_, _ = fmt.Fprintf(w, "A newer stable k0s exists: %s (current default: %s)\n", stableTag, current)
				} else {
					_, _ = fmt.Fprintln(w, "Default k0s version is up to date with stable.")
				}
			} else {
				_, _ = fmt.Fprintf(w, "Failed to check latest k0s version: %v\n", err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().Bool("check-latest", false, "check for the latest stable k0s version")
}
