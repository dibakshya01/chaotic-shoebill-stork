package cli

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/catalog"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/safety"
	"github.com/spf13/cobra"
)

// heldProber reports steady state as held (a CLI preview demonstrates the policy gates,
// not live signal probing).
type heldProber struct{}

func (heldProber) Probe(context.Context, []model.Probe, map[string]any) safety.SteadyStateProof {
	return safety.SteadyStateProof{Held: true}
}

func registerRunCmd(root *cobra.Command, stdout, stderr io.Writer) {
	run := &cobra.Command{Use: "run", Short: "Plan and evaluate experiment runs"}
	run.AddCommand(newRunPlanCmd(stdout, stderr))
	root.AddCommand(run)
}

func newRunPlanCmd(stdout, stderr io.Writer) *cobra.Command {
	var catDir, expName, env, caps string
	var tier, pods, dur, traffic, approvals int
	var payments bool
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Evaluate the Safety Governor for a run WITHOUT detonating (all 10 gates)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			exps, err := catalog.LoadExperiments(catDir)
			if err != nil {
				return err
			}
			var exp *model.Experiment
			for i := range exps {
				if exps[i].Meta.Name == expName {
					exp = &exps[i]
				}
			}
			if exp == nil {
				return fmt.Errorf("experiment %q not found in %s (try: shoebill catalog list)", expName, catDir)
			}

			run := model.Run{
				ExperimentRef: expName,
				Target: model.Target{
					Service:   model.Service{ID: "preview", Name: "preview", Tier: model.Tier(tier), Team: "service-owner", OnPaymentsPath: payments},
					Namespace: "preview", Env: model.Env(env), Ring: model.RingCanary,
				},
				Scope: model.Scope{PodCount: pods, PctOfReplicas: 5, DurationSec: dur, PctTraffic: traffic},
			}
			teams := []string{"service-owner", "appsec"}
			for i := 0; i < approvals; i++ {
				team := fmt.Sprintf("team-%d", i)
				if i < len(teams) {
					team = teams[i]
				}
				run.Approvals = append(run.Approvals, model.Approval{Role: "approver", Team: team, Approver: "cli", At: time.Now()})
			}
			var granted []string
			for _, c := range strings.Split(caps, ",") {
				if c = strings.TrimSpace(c); c != "" {
					granted = append(granted, c)
				}
			}

			gov := newPreviewGovernor(exps)
			dec, err := gov.Evaluate(context.Background(), safety.RunRequest{Experiment: *exp, Run: run, GrantedCapabilities: granted, Now: time.Now()})
			if err != nil {
				return err
			}
			renderDecision(stdout, *exp, dec)
			if !dec.Allowed {
				return fmt.Errorf("refused by the safety governor")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&catDir, "catalog", "catalog", "catalog directory")
	cmd.Flags().StringVar(&expName, "experiment", "", "experiment name (required)")
	cmd.Flags().StringVar(&env, "env", "prod-canary", "environment")
	cmd.Flags().IntVar(&tier, "tier", 1, "service tier (1-3)")
	cmd.Flags().IntVar(&pods, "pods", 1, "requested pod count")
	cmd.Flags().IntVar(&dur, "duration", 60, "requested duration (seconds)")
	cmd.Flags().IntVar(&traffic, "traffic", 1, "requested traffic percent")
	cmd.Flags().IntVar(&approvals, "approvals", 2, "number of approvals (distinct teams)")
	cmd.Flags().StringVar(&caps, "cap", "prod-injection", "granted capabilities (comma-separated)")
	cmd.Flags().BoolVar(&payments, "payments", false, "target is on the payments path")
	_ = cmd.MarkFlagRequired("experiment")
	return cmd
}

func newPreviewGovernor(exps []model.Experiment) *safety.DefaultGovernor {
	_, priv, _ := ed25519.GenerateKey(nil)
	promo := &safety.MapPromotionHistory{}
	now := time.Now()
	for _, e := range exps {
		promo.Record(e.Meta.Name, model.EnvPlayground, now.Add(-24*time.Hour))
		promo.Record(e.Meta.Name, model.EnvStage, now.Add(-24*time.Hour))
		promo.Record(e.Meta.Name, model.EnvPreprod, now.Add(-24*time.Hour))
	}
	providers := safety.Providers{
		KillSwitch: safety.StaticKillSwitch{}, Incidents: safety.NoIncidents{}, Blackout: safety.StaticBlackout{},
		Promotions: promo, Concurrency: safety.FreeConcurrency{Max: 5}, FaultDomains: safety.NoFaultDomainClash{},
		SteadyState: heldProber{},
	}
	return safety.NewGovernor(providers, "preview", priv)
}

func renderDecision(w io.Writer, exp model.Experiment, dec safety.Decision) {
	byGate := map[safety.GateID]safety.Refusal{}
	for _, r := range dec.Refusals {
		if _, ok := byGate[r.Gate]; !ok {
			byGate[r.Gate] = r
		}
	}
	fmt.Fprintf(w, "\n  %s%s%s  %s%s%s\n", cBold, exp.Meta.Name, cReset, cDim, exp.Meta.Title, cReset)
	if dec.Allowed {
		fmt.Fprintf(w, "  %s✓ ALLOWED%s — a signed capability would be minted\n\n", cGreen+cBold, cReset)
	} else {
		fmt.Fprintf(w, "  %s⛔ REFUSED%s — %d gate(s) failed\n\n", cRed+cBold, cReset, len(byGate))
	}
	for _, g := range safety.Gates {
		if rf, bad := byGate[g]; bad {
			fmt.Fprintf(w, "  %s✗%s %-14s %s\n       %s→ %s%s\n", cRed, cReset, g, rf.Reason, cDim, rf.Remedy, cReset)
		} else {
			fmt.Fprintf(w, "  %s✓%s %-14s %spassed%s\n", cGreen, cReset, g, cDim, cReset)
		}
	}
	fmt.Fprintf(w, "\n  %spolicy %s · envelope: %d pod(s), %ds, ≤%d%% traffic%s\n\n",
		cDim, dec.PolicyVersion, dec.Constraints.MaxPods, dec.Constraints.MaxDurationSeconds, dec.Constraints.MaxTrafficPct, cReset)
}
