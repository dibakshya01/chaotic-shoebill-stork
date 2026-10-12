package cli

import (
	"io"

	"github.com/spf13/cobra"
)

// registerCatalogCmd and registerServeCmd are filled in by later milestones
// (catalog = M6, serve = M7). They are defined as no-ops here so the command tree
// compiles as the project is built incrementally; each is replaced by its real
// implementation in its own file.

func registerCatalogCmd(_ *cobra.Command, _, _ io.Writer) {}

func registerServeCmd(_ *cobra.Command, _, _ io.Writer) {}
