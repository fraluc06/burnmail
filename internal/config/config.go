// Package config holds build-time configuration shared across the app.
package config

// Version is injected at build time via -ldflags "-X burnmail/internal/config.Version=...".
// The git tag is the single source of truth; local builds fall back to "dev".
var Version = "dev"
