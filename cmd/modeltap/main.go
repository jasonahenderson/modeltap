package main

import "fmt"

// version is set via ldflags at build time.
// Example: go build -ldflags "-X main.version=1.0.0" ./cmd/modeltap/
var version = "dev"

func main() {
	fmt.Printf("modeltap %s\n", version)
}
