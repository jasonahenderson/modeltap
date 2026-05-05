package main

import (
	"fmt"
	"os"

	"github.com/jasonahenderson/modeltap/internal/cli"
	"github.com/jasonahenderson/modeltap/internal/dotenv"
)

// version is set via ldflags at build time.
// Example: go build -ldflags "-X main.version=1.0.0" ./cmd/modeltap/
var version = "dev"

func main() {
	// PATCH-0007: populate os.Getenv from ./.env or ~/.modeltap/.env
	// before anything reads config — Viper's AutomaticEnv and
	// PATCH-0004's env: resolver both depend on values already being
	// present in the process environment.
	if err := dotenv.Load(os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	rootCmd := cli.NewRootCommand(version)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
