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
	registerCatalogCmd(root, stdout, stderr)
	registerRunCmd(root, stdout, stderr)
	registerServeCmd(root, stdout, stderr)
}
