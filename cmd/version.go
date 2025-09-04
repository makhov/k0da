package cmd

import (
	"fmt"

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
		fmt.Fprintf(w, "k0da %s", Version)
		if Commit != "" {
			fmt.Fprintf(w, " (commit %s)", Commit)
		}
		if BuildDate != "" {
			fmt.Fprintf(w, " built %s", BuildDate)
		}
		fmt.Fprintln(w)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
