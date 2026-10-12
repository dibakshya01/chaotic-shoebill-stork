package safety

import (
	"fmt"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// PolicyVersion is recorded in every Decision for reproducibility. Bump it whenever
// the envelopes below change, so a receipt names the exact policy that ran.
const PolicyVersion = "blast-radius-v1"

const unlimited = 1 << 30

// envEnvelope returns the maximum blast radius permitted in an environment. The
// envelope tightens with environment impact; prod-canary is the strictest.
func envEnvelope(env model.Env, tier model.Tier) Constraints {
	switch env {
	case model.EnvLocal, model.EnvChaosLab:
		return Constraints{MaxPods: unlimited, MaxPctReplicas: 100, MaxDurationSeconds: 3600, MaxTrafficPct: 100}
	case model.EnvPlayground:
		// Near-unrestricted, still time-bounded. No real customer traffic.
		return Constraints{MaxPods: unlimited, MaxPctReplicas: 100, MaxDurationSeconds: 3600, MaxTrafficPct: 0}
	case model.EnvStage:
		return Constraints{MaxPods: unlimited, MaxPctReplicas: 50, MaxDurationSeconds: 900, MaxTrafficPct: 0}
	case model.EnvPreprod:
		return Constraints{MaxPods: unlimited, MaxPctReplicas: 20, MaxDurationSeconds: 300, MaxTrafficPct: 0}
	case model.EnvProdCanary:
		// The tightest envelope in the system: one pod, one minute, ≤1% traffic.
		return Constraints{MaxPods: 1, MaxPctReplicas: 5, MaxDurationSeconds: 60, MaxTrafficPct: 1}
	case model.EnvProdRing:
		return Constraints{MaxPods: unlimited, MaxPctReplicas: 5, MaxDurationSeconds: 60, MaxTrafficPct: 5}
	default:
		// Unknown environment: deny everything by returning a zero envelope.
		return Constraints{}
	}
}

// tightest returns the per-dimension minimum of two constraints, treating a 0 in the
// experiment's own blast radius as "no additional limit" (the envelope governs).
func tightest(env Constraints, exp model.BlastRadius) Constraints {
	minv := func(a, b int) int {
		if b <= 0 {
			return a
		}
		if b < a {
			return b
		}
		return a
	}
	return Constraints{
		MaxPods:            minv(env.MaxPods, exp.MaxPods),
		MaxPctReplicas:     minv(env.MaxPctReplicas, exp.MaxPctReplicas),
		MaxDurationSeconds: minv(env.MaxDurationSeconds, exp.MaxDurationSeconds),
		MaxTrafficPct:      minv(env.MaxTrafficPct, exp.MaxTrafficPct),
	}
}

// evalBudget computes the effective constraints and any blast-radius violations.
func evalBudget(req RunRequest) (Constraints, []Refusal) {
	env := req.Run.Target.Env
	tier := req.Run.Target.Service.Tier
	c := tightest(envEnvelope(env, tier), req.Experiment.BlastRadius)
	var refusals []Refusal

	add := func(reason, remedy string) {
		refusals = append(refusals, Refusal{Gate: GateBudget, Reason: reason, Remedy: remedy})
	}

	// Payments path in production: observation only, never injection.
	if env.IsProd() && req.Run.Target.Service.OnPaymentsPath && req.Experiment.Injection.Mode != model.ModeObserveOnly {
		add("payments-path service in production may only run observe_only experiments",
			"set the experiment to observe_only, or run it in preprod with production-shaped traffic")
	}

	// Observe-only experiments carry no blast radius — the scope checks don't apply.
	if req.Experiment.Injection.Mode == model.ModeObserveOnly {
		return c, refusals
	}

	// Inject-mode experiments must be reversible, especially in production.
	if !req.Experiment.Injection.Reversible {
		add("inject-mode experiment is not declared reversible",
			"declare reversible: true and a revertVerification probe, or use observe_only")
	}

	s := req.Run.Scope
	if s.PodCount > c.MaxPods {
		add(fmt.Sprintf("pod count %d exceeds max %d for %s", s.PodCount, c.MaxPods, env),
			fmt.Sprintf("reduce pods to ≤ %d", c.MaxPods))
	}
	if s.PctOfReplicas > c.MaxPctReplicas {
		add(fmt.Sprintf("%d%% of replicas exceeds max %d%% for %s", s.PctOfReplicas, c.MaxPctReplicas, env),
			fmt.Sprintf("reduce to ≤ %d%% of replicas", c.MaxPctReplicas))
	}
	if s.DurationSec > c.MaxDurationSeconds {
		add(fmt.Sprintf("duration %ds exceeds max %ds for %s", s.DurationSec, c.MaxDurationSeconds, env),
			fmt.Sprintf("reduce duration to ≤ %ds, or request a budget exception", c.MaxDurationSeconds))
	}
	if s.PctTraffic > c.MaxTrafficPct {
		add(fmt.Sprintf("%d%% traffic exceeds max %d%% for %s", s.PctTraffic, c.MaxTrafficPct, env),
			fmt.Sprintf("reduce to ≤ %d%% traffic (synthetic only below prod)", c.MaxTrafficPct))
	}
	return c, refusals
}

// requiredApprovals returns how many approvals an environment needs and whether they
// must come from distinct teams.
func requiredApprovals(env model.Env) (count int, distinctTeams bool) {
	switch env {
	case model.EnvLocal, model.EnvChaosLab, model.EnvPlayground:
		return 0, false
	case model.EnvStage:
		return 1, false
	case model.EnvPreprod:
		return 2, false
	case model.EnvProdCanary, model.EnvProdRing:
		return 2, true
	default:
		return 2, true
	}
}

// requiredCapabilities returns the capability grants an environment + experiment need.
func requiredCapabilities(exp model.Experiment, env model.Env) []string {
	req := map[string]struct{}{}
	for _, c := range exp.Capabilities {
		req[c] = struct{}{}
	}
	if env.IsProd() {
		req["prod-injection"] = struct{}{}
	}
	if exp.Family == model.FamilyAI && exp.Injection.Mode == model.ModeInject {
		req["ai-injection"] = struct{}{}
	}
	out := make([]string, 0, len(req))
	for c := range req {
		out = append(out, c)
	}
	return out
}
