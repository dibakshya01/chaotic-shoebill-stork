// Package cli assembles the shoebill command tree. Subcommands are added in
// their own files (catalog, run, score, evidence, serve, demo). This file holds
// the root command and Execute.
package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/buildinfo"
	"github.com/spf13/cobra"
)

// Execute runs the shoebill CLI with the given args.
func Execute(ctx context.Context, args []string) error {
	root := newRootCmd(os.Stdout, os.Stderr)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

func newRootCmd(stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:   "shoebill",
		Short: "Security Chaos Engineering — prove your security controls actually work",
		Long: "shoebill is a Security Chaos Engineering platform. It treats every security\n" +
			"control as a testable hypothesis: it safely detonates a bounded, reversible,\n" +
			"pre-declared adversarial condition and asserts whether the control prevented,\n" +
			"detected, alerted, and paged within its SLO — then signs the evidence and\n" +
			"rolls it into a per-service control-efficacy scorecard.\n\n" +
			"Nothing detonates without a Safety Governor decision. See CHARTER.md.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildinfo.Version,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)

	root.AddCommand(newVersionCmd(stdout))
	// Further subcommands are registered here as they are built:
	//   catalog, run, score, evidence, serve, demo
	registerCommands(root, stdout, stderr)
	return root
}

func newVersionCmd(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(stdout, buildinfo.String())
			return nil
		},
	}
}
