package main

import (
	"os"

	"github.com/jasonahenderson/modeltap/internal/cli"
)

// version is set via ldflags at build time.
// Example: go build -ldflags "-X main.version=1.0.0" ./cmd/modeltap/
var version = "dev"

func main() {
	rootCmd := cli.NewRootCommand(version)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
