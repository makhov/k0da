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
		fmt.Printf("warn: failed to extract plugins: %v\n", err)
	}

	if err := cmd.Execute(); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}
