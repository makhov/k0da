package main

import (
	"fmt"
	"github.com/makhov/k0da/internal/plugins"
	"os"

	"github.com/makhov/k0da/cmd"
)

func main() {
	_, err := plugins.ExtractPlugins()
	if err != nil {
		fmt.Println("warn! failed to extract plugins: %w", err)
	}

	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
