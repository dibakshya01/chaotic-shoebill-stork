package safety

import (
	"context"
	"fmt"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// defaultPromotionWindowDays is the default freshness window for the promotion gate
// when an experiment does not specify its own.
const defaultPromotionWindowDays = 14

func gateKillSwitch(ctx context.Context, p Providers) []Refusal {
	if p.KillSwitch != nil && p.KillSwitch.Engaged(ctx) {
		return []Refusal{{
			Gate:   GateKillSwitch,
			Reason: "the global kill switch is engaged",
			Remedy: "clear the kill switch before running any experiment",
		}}
	}
	return nil
}

func gateSEV(ctx context.Context, req RunRequest, p Providers) []Refusal {
	if p.Incidents == nil {
		return nil
	}
	team := req.Run.Target.Service.Team
	if active, id := p.Incidents.ActiveSev(ctx, team); active {
		return []Refusal{{
			Gate:   GateSEVActive,
			Reason: fmt.Sprintf("an active incident (%s) is open for team %q", id, team),
			Remedy: "wait until the incident is resolved; chaos must never run during a real incident",
		}}
	}
	return nil
}

func gateBlackout(ctx context.Context, req RunRequest, p Providers) []Refusal {
	if p.Blackout == nil {
		return nil
	}
	in, window, next := p.Blackout.InWindow(ctx, req.Run.Target.Env, req.Run.Target.Service.Team, req.Now)
	if in {
		return []Refusal{{
			Gate:         GateBlackout,
			Reason:       fmt.Sprintf("now is inside blackout window %q", window),
			Remedy:       "run outside the freeze window",
			NextEligible: next,
		}}
	}
	return nil
}

// lowerEnvs returns the environments that must have passed before the target env.
func lowerEnvs(exp model.Experiment, target model.Env) []model.Env {
	if len(exp.Promotion.RequiresPassIn) > 0 {
		return exp.Promotion.RequiresPassIn
	}
	all := []model.Env{model.EnvPlayground, model.EnvStage, model.EnvPreprod, model.EnvProdCanary}
	var out []model.Env
	for _, e := range all {
		if e.Rank() < target.Rank() {
			out = append(out, e)
		}
	}
	return out
}

func gatePromotion(ctx context.Context, req RunRequest, p Providers) []Refusal {
	env := req.Run.Target.Env
	// Lowest environments need no promotion history.
	if env == model.EnvLocal || env == model.EnvChaosLab || env == model.EnvPlayground {
		return nil
	}
	if p.Promotions == nil {
		return []Refusal{{
			Gate:   GatePromotion,
			Reason: "no promotion history available to confirm lower-environment passes",
			Remedy: "run the experiment in lower environments first",
		}}
	}
	windowDays := req.Experiment.Promotion.WithinDays
	if windowDays <= 0 {
		windowDays = defaultPromotionWindowDays
	}
	cutoff := req.Now.Add(-time.Duration(windowDays) * 24 * time.Hour)
	var refusals []Refusal
	for _, lower := range lowerEnvs(req.Experiment, env) {
		at, ok := p.Promotions.LastPass(ctx, req.Experiment.Meta.Name, lower)
		if !ok || at.Before(cutoff) {
			refusals = append(refusals, Refusal{
				Gate:   GatePromotion,
				Reason: fmt.Sprintf("experiment has not PASSed in %s within %d days", lower, windowDays),
				Remedy: fmt.Sprintf("run it in %s and get a PASS first (the first prod run is never a first run)", lower),
			})
		}
	}
	return refusals
}

func gateCapability(req RunRequest) []Refusal {
	granted := map[string]struct{}{}
	for _, c := range req.GrantedCapabilities {
		granted[c] = struct{}{}
	}
	var refusals []Refusal
	for _, need := range requiredCapabilities(req.Experiment, req.Run.Target.Env) {
		if _, ok := granted[need]; !ok {
			refusals = append(refusals, Refusal{
				Gate:   GateCapability,
				Reason: fmt.Sprintf("missing required capability %q", need),
				Remedy: fmt.Sprintf("request the %q capability grant from the approving authority", need),
			})
		}
	}
	return refusals
}

func gateApproval(req RunRequest) []Refusal {
	need, distinct := requiredApprovals(req.Run.Target.Env)
	if need == 0 {
		return nil
	}
	got := len(req.Run.Approvals)
	var refusals []Refusal
	if got < need {
		refusals = append(refusals, Refusal{
			Gate:   GateApproval,
			Reason: fmt.Sprintf("%d approval(s) present, %d required for %s", got, need, req.Run.Target.Env),
			Remedy: fmt.Sprintf("obtain %d approval(s)", need-got),
		})
	}
	if distinct && req.Run.DistinctApprovalTeams() < 2 {
		refusals = append(refusals, Refusal{
			Gate:   GateApproval,
			Reason: "production requires approvals from two distinct teams",
			Remedy: "obtain a second approval from a different team (e.g. service owner + AppSec)",
		})
	}
	return refusals
}

func gateFaultDomain(ctx context.Context, req RunRequest, p Providers) []Refusal {
	if !req.Experiment.BlastRadius.RequiresDistinctFaultDomains || p.FaultDomains == nil {
		return nil
	}
	if clash, with := p.FaultDomains.Clash(ctx, req.Run.Target.FaultDomainID); clash {
		return []Refusal{{
			Gate:   GateFaultDomain,
			Reason: fmt.Sprintf("target shares fault domain %q with in-flight run %s", req.Run.Target.FaultDomainID, with),
			Remedy: "wait for the other run, or target a distinct fault domain",
		}}
	}
	return nil
}

func gateConcurrency(ctx context.Context, req RunRequest, p Providers) []Refusal {
	if p.Concurrency == nil {
		return nil
	}
	if free, used, max := p.Concurrency.Slot(ctx, req); !free {
		return []Refusal{{
			Gate:   GateConcurrency,
			Reason: fmt.Sprintf("no concurrency slot free (%d of %d in use)", used, max),
			Remedy: "wait for an in-flight run to finish",
		}}
	}
	return nil
}

func gateSteadyState(ctx context.Context, req RunRequest, p Providers) (SteadyStateProof, []Refusal) {
	probes := req.Experiment.Hypothesis.SteadyState
	if p.SteadyState == nil || len(probes) == 0 {
		// No prober or no probes declared: cannot assert steady state. We record an
		// empty, not-held proof and refuse — injecting into an unknown state produces
		// a garbage verdict and risks a real incident.
		if len(probes) == 0 {
			return SteadyStateProof{Held: false}, []Refusal{{
				Gate:   GateSteadyState,
				Reason: "the experiment declares no steady-state probe",
				Remedy: "add at least one steady-state probe so the hypothesis is measurable",
			}}
		}
		return SteadyStateProof{Held: false}, []Refusal{{
			Gate:   GateSteadyState,
			Reason: "no steady-state prober configured",
			Remedy: "configure a signal source to evaluate the steady-state probes",
		}}
	}
	proof := p.SteadyState.Probe(ctx, probes, req.Run.Parameters)
	if !proof.Held {
		return proof, []Refusal{{
			Gate:   GateSteadyState,
			Reason: "the system is not in steady state right now",
			Remedy: "resolve the failing steady-state probe before injecting",
		}}
	}
	return proof, nil
}
