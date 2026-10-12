package cli

import (
	"fmt"
	"io"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/demo"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/findings"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/store"
	"github.com/spf13/cobra"
)

func newDemoCmd(stdout io.Writer) *cobra.Command {
	var dir string
	var seed bool
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Run the full detonate -> verify -> score -> sign loop in-process",
		Long: "demo wires the entire platform in-process and runs a realistic matrix of\n" +
			"experiments against a mock environment — producing real signed evidence and a\n" +
			"populated control-efficacy scorecard, including genuine FAIL_OPEN findings.\n" +
			"No Kubernetes, no network, no external dependencies.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := demo.Seed(dir)
			if err != nil {
				return fmt.Errorf("demo: %w", err)
			}
			RenderScorecard(stdout, res.Scorecard)

			st, err := store.Load(demo.StatePath(dir))
			if err == nil {
				var lines []string
				for _, f := range st.ListFailOpen() {
					lines = append(lines, formatFindingLine(f))
				}
				RenderFailOpen(stdout, lines)
			}
			fmt.Fprintf(stdout, "\n  %d runs executed · %d fail-open.\n", res.Runs, res.FailOpen)
			fmt.Fprintf(stdout, "  Signed evidence: %s\n", demo.EvidenceDir(dir))
			fmt.Fprintf(stdout, "  Next:  %sshoebill evidence verify%s   %sshoebill serve --demo%s\n\n",
				cBold, cReset, cBold, cReset)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".shoebill", "directory for state + evidence")
	cmd.Flags().BoolVar(&seed, "seed", true, "seed fresh demo data")
	return cmd
}

func formatFindingLine(f findings.Finding) string {
	sev := map[findings.Severity]string{
		findings.Critical: cRedBg + " CRITICAL " + cReset,
		findings.High:     cRed + "HIGH" + cReset,
		findings.Medium:   cAmber + "MED " + cReset,
		findings.Low:      cDim + "LOW " + cReset,
	}[f.Severity]
	return fmt.Sprintf("%s %s · %s%s%s · %s\n       %s%s%s  [%s]",
		sev, f.ServiceName, cBold, f.Family.Title(), cReset, f.Title,
		cDim, f.Detail, cReset, joinIDs(f))
}

func joinIDs(f findings.Finding) string {
	ids := append([]string{}, f.AttackIDs...)
	ids = append(ids, f.OwaspIDs...)
	ids = append(ids, f.ControlFamilies...)
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += " · "
		}
		out += id
	}
	return out
}
