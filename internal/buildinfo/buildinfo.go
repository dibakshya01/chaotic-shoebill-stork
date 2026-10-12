// Package buildinfo carries version metadata stamped in at build time via -ldflags.
package buildinfo

import "fmt"

// These are overridden at build time (see the Makefile).
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a human-readable one-line version string.
func String() string {
	return fmt.Sprintf("chaotic-shoebill-stork %s (commit %s, built %s)", Version, Commit, Date)
}
