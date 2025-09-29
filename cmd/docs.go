package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var docsCmd = &cobra.Command{
	Use:   "docs {markdown|man} {<dir>}",
	Short: "Generate k0da command documentation",
	Args:  cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := args[1]
		switch args[0] {
		case "markdown":
			return doc.GenMarkdownTree(rootCmd, dir)
		case "man":
			return doc.GenManTree(rootCmd, &doc.GenManHeader{Title: "k0da", Section: "1"}, dir)
		}
		return errors.New("invalid format")
	},
}

func init() {
	rootCmd.AddCommand(docsCmd)
}
