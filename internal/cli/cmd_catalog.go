package cli

import (
	"fmt"
	"io"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/catalog"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func registerCatalogCmd(root *cobra.Command, stdout, stderr io.Writer) {
	var dir string
	cat := &cobra.Command{
		Use:   "catalog",
		Short: "Browse and validate the experiment catalog",
	}
	cat.PersistentFlags().StringVar(&dir, "dir", "catalog", "catalog directory")

	validate := &cobra.Command{
		Use:   "validate [dir]",
		Short: "Validate every experiment and the service registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d := dir
			if len(args) == 1 {
				d = args[0]
			}
			exps, err := catalog.LoadExperiments(d)
			if err != nil {
				return err
			}
			total, bad := 0, 0
			for _, name := range catalog.DuplicateNames(exps) {
				bad++
				fmt.Fprintf(stderr, "%s✗ duplicate experiment name %q%s\n", cRed, name, cReset)
			}
			for _, e := range exps {
				total++
				if errs := catalog.Validate(e); len(errs) > 0 {
					bad++
					fmt.Fprintf(stderr, "%s✗ %s%s\n", cRed, e.Meta.Name, cReset)
					for _, er := range errs {
						fmt.Fprintf(stderr, "      - %v\n", er)
					}
				} else {
					fmt.Fprintf(stdout, "%s✓%s %-34s %s%s%s\n", cGreen, cReset, e.Meta.Name, cDim, e.Family.Title(), cReset)
				}
			}
			svcs, err := catalog.LoadServices(d)
			if err != nil {
				return err
			}
			if serrs := catalog.ValidateServices(svcs); len(serrs) > 0 {
				bad++
				fmt.Fprintf(stderr, "%s✗ services.yaml%s\n", cRed, cReset)
				for _, er := range serrs {
					fmt.Fprintf(stderr, "      - %v\n", er)
				}
			} else if len(svcs) > 0 {
				fmt.Fprintf(stdout, "%s✓%s services.yaml (%d services)\n", cGreen, cReset, len(svcs))
			}
			fmt.Fprintf(stdout, "\n%d experiments, %d invalid.\n", total, bad)
			if bad > 0 {
				return fmt.Errorf("%d catalog item(s) failed validation", bad)
			}
			return nil
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List catalog experiments",
		RunE: func(cmd *cobra.Command, _ []string) error {
			exps, err := catalog.LoadExperiments(dir)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "%s%-34s %-22s %s%s\n", cBold, "ID", "FAMILY", "TITLE", cReset)
			for _, e := range exps {
				ids := ""
				if len(e.AttackIDs) > 0 {
					ids = e.AttackIDs[0]
				}
				fmt.Fprintf(stdout, "%-34s %-22s %s %s(%s)%s\n", e.Meta.Name, e.Family.Title(), e.Meta.Title, cDim, ids, cReset)
			}
			fmt.Fprintf(stdout, "\n%d experiments across %d families.\n", len(exps), countFamilies(exps))
			return nil
		},
	}

	show := &cobra.Command{
		Use:   "show <name>",
		Short: "Show one experiment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			exps, err := catalog.LoadExperiments(dir)
			if err != nil {
				return err
			}
			for _, e := range exps {
				if e.Meta.Name == args[0] {
					b, _ := yaml.Marshal(e)
					fmt.Fprintln(stdout, string(b))
					return nil
				}
			}
			return fmt.Errorf("experiment %q not found in %s", args[0], dir)
		},
	}

	cat.AddCommand(validate, list, show)
	root.AddCommand(cat)
}

func countFamilies(exps []model.Experiment) int {
	seen := map[model.Family]bool{}
	for _, e := range exps {
		seen[e.Family] = true
	}
	return len(seen)
}
