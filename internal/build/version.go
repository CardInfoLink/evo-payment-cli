// Package build holds version information injected at compile time via ldflags.
//
// Build with:
//
//	go build -ldflags "-X github.com/evopayment/evo-cli/internal/build.Version=1.0.0 -X github.com/evopayment/evo-cli/internal/build.Date=2024-01-01"
package build

// Version is the CLI version, injected via ldflags at build time.
var Version = "dev"

// Date is the build date, injected via ldflags at build time.
var Date = "unknown"
