package cli

import (
	"io"

	"github.com/spf13/cobra"
)

// registerServeCmd is filled in by M7 (the API + embedded dashboard). Defined as a
// no-op here so the command tree compiles as the project is built incrementally; it is
// replaced by its real implementation in cmd_serve.go.
func registerServeCmd(_ *cobra.Command, _, _ io.Writer) {}
