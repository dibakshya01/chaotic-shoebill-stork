package cli

import (
	"io"

	"github.com/spf13/cobra"
)

// registerCommands wires the feature subcommands onto the root.
func registerCommands(root *cobra.Command, stdout, stderr io.Writer) {
	root.AddCommand(newDemoCmd(stdout))
	root.AddCommand(newScoreCmd(stdout))
	root.AddCommand(newEvidenceCmd(stdout, stderr))
	// catalog (M6) and serve (M7) are registered as they land.
	registerCatalogCmd(root, stdout, stderr)
	registerServeCmd(root, stdout, stderr)
}
