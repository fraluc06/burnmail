package main

import (
	"burnmail/cmd"
)

// Version is injected at build time via -ldflags "-X main.Version=...".
// The git tag is the single source of truth; local builds fall back to "dev".
var Version = "dev"

func main() {
	cmd.Version = Version
	cmd.Execute()
}
