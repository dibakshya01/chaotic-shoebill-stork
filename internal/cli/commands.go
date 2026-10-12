package cli

import (
	"io"

	"github.com/spf13/cobra"
)

// registerCommands wires the feature subcommands onto the root. They are added
// here as each milestone lands (catalog, run, score, evidence, serve, demo).
func registerCommands(root *cobra.Command, stdout, stderr io.Writer) {
	// populated in later milestones
}
