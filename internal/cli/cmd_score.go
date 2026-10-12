package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/demo"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/store"
	"github.com/spf13/cobra"
)

func newScoreCmd(stdout io.Writer) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:   "score",
		Short: "Print the control-efficacy scorecard from stored run results",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st, err := store.Load(demo.StatePath(dir))
			if err != nil {
				return err
			}
			if len(st.ListServices()) == 0 {
				fmt.Fprintf(stdout, "No data yet. Run %sshoebill demo%s to seed a scorecard.\n", cBold, cReset)
				return nil
			}
			RenderScorecard(stdout, st.Scorecard(time.Now()))
			fmt.Fprintln(stdout)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".shoebill", "directory holding state")
	return cmd
}
