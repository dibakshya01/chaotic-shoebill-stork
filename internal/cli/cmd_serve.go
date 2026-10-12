package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/api"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/catalog"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/demo"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/store"
	"github.com/spf13/cobra"
)

func registerServeCmd(root *cobra.Command, stdout, stderr io.Writer) {
	var addr, dir, catDir string
	var useDemo, insecureRemote bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the dashboard + REST API (localhost-only by default)",
		Long: "serve starts the operator console: the embedded dashboard and the read + safety-\n" +
			"preview + validate API. It binds to localhost and is unauthenticated by design — it\n" +
			"is a local console, not a multi-tenant control surface. Execution stays CLI/operator-first.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return fmt.Errorf("invalid --addr %q: %w", addr, err)
			}
			if !isLoopback(host) && !insecureRemote {
				return fmt.Errorf("refusing to bind non-loopback address %q: the API is unauthenticated; "+
					"pass --insecure-remote to override (unsupported)", addr)
			}

			if useDemo {
				if _, serr := statState(dir); serr != nil {
					fmt.Fprintf(stdout, "seeding demo data in %s …\n", dir)
					if _, derr := demo.Seed(dir); derr != nil {
						return fmt.Errorf("seed demo: %w", derr)
					}
				}
			}
			st, err := store.Load(demo.StatePath(dir))
			if err != nil {
				return err
			}
			exps, err := catalog.LoadExperiments(catDir)
			if err != nil || len(exps) == 0 {
				exps = fallbackExperiments(st) // keep the preview usable even without a catalog dir
			}

			srv := api.New(st, exps)
			httpSrv := &http.Server{
				Addr: addr, Handler: srv.Handler(),
				ReadHeaderTimeout: 5 * time.Second,
			}
			errCh := make(chan error, 1)
			go func() { errCh <- httpSrv.ListenAndServe() }()

			fmt.Fprintf(stdout, "%s%s  shoebill dashboard%s  →  http://%s\n", cBold, cCyan, cReset, addr)
			fmt.Fprintf(stdout, "  %d experiments · %d services loaded. Ctrl-C to stop.\n", len(exps), len(st.ListServices()))

			select {
			case <-cmd.Context().Done():
				shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = httpSrv.Shutdown(shutCtx)
				fmt.Fprintln(stdout, "\nstopped.")
				return nil
			case err := <-errCh:
				if err == http.ErrServerClosed {
					return nil
				}
				return err
			}
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:8099", "address to bind")
	cmd.Flags().StringVar(&dir, "dir", ".shoebill", "state + evidence directory")
	cmd.Flags().StringVar(&catDir, "catalog", "catalog", "experiment catalog directory")
	cmd.Flags().BoolVar(&useDemo, "demo", false, "seed + serve demo data if none exists")
	cmd.Flags().BoolVar(&insecureRemote, "insecure-remote", false, "allow binding a non-loopback address (unsupported)")
	root.AddCommand(cmd)
}

func isLoopback(host string) bool {
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// statState reports whether a state file exists (returns its presence).
func statState(dir string) (bool, error) {
	st, err := store.Load(demo.StatePath(dir))
	if err != nil {
		return false, err
	}
	if len(st.ListServices()) == 0 {
		return false, fmt.Errorf("no state")
	}
	return true, nil
}

// fallbackExperiments synthesizes a minimal experiment list from stored runs so the
// Experiments/Safety-Preview screens are not empty when no catalog dir is present.
func fallbackExperiments(st *store.Memory) []model.Experiment {
	seen := map[string]bool{}
	var out []model.Experiment
	for _, r := range st.ListRuns() {
		if r.Experiment == "" || seen[r.Experiment] {
			continue
		}
		seen[r.Experiment] = true
		out = append(out, model.Experiment{
			Meta:   model.ExperimentMeta{Name: r.Experiment, Version: 1, Title: r.Title},
			Family: r.Family,
			Injection: model.Injection{
				Detonator: "mock", Mode: model.ModeInject, Reversible: true,
				Duration: model.Duration(45 * time.Second),
			},
			Hypothesis:  model.Hypothesis{SteadyState: []model.Probe{{Source: model.SourceMock, Query: "up", Expect: model.Expectation{Type: model.ExpectPresent}}}},
			Assertions:  []model.Assertion{{Stage: model.Prevented, Source: model.SourceMock, Query: "x", Expect: model.Expectation{Type: model.ExpectAbsent}}},
			BlastRadius: model.BlastRadius{MaxPods: 1, MaxDurationSeconds: 60},
			AttackIDs:   []string{"T1078"},
		})
	}
	return out
}
