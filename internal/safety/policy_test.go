package safety

import (
	"context"
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

func TestEnvEnvelopes(t *testing.T) {
	pc := envEnvelope(model.EnvProdCanary, 1)
	if pc.MaxPods != 1 || pc.MaxDurationSeconds != 60 || pc.MaxTrafficPct != 1 || pc.MaxPctReplicas != 5 {
		t.Fatalf("prod-canary envelope wrong: %+v", pc)
	}
	if envEnvelope(model.EnvPlayground, 1).MaxTrafficPct != 0 {
		t.Fatal("playground should allow no real traffic")
	}
	if envEnvelope(model.Env("nonsense"), 1) != (Constraints{}) {
		t.Fatal("unknown env must return a zero (deny-all) envelope")
	}
}

func TestEvalBudgetObserveOnlyAllowedOnPaymentsProd(t *testing.T) {
	req := RunRequest{
		Experiment: model.Experiment{Injection: model.Injection{Mode: model.ModeObserveOnly}},
		Run:        model.Run{Target: model.Target{Env: model.EnvProdCanary, Service: model.Service{Tier: 1, OnPaymentsPath: true}}},
	}
	_, refs := evalBudget(req)
	if len(refs) != 0 {
		t.Fatalf("observe_only on payments prod must be allowed, got %+v", refs)
	}
}

func TestEvalBudgetInjectOnPaymentsProdRefused(t *testing.T) {
	req := RunRequest{
		Experiment: model.Experiment{Injection: model.Injection{Mode: model.ModeInject, Reversible: true}},
		Run:        model.Run{Target: model.Target{Env: model.EnvProdCanary, Service: model.Service{Tier: 1, OnPaymentsPath: true}}},
	}
	_, refs := evalBudget(req)
	if len(refs) == 0 {
		t.Fatal("inject on payments prod must be refused")
	}
}

func TestEvalBudgetInjectMustBeReversible(t *testing.T) {
	req := RunRequest{
		Experiment: model.Experiment{Injection: model.Injection{Mode: model.ModeInject, Reversible: false}},
		Run:        model.Run{Target: model.Target{Env: model.EnvStage}},
	}
	_, refs := evalBudget(req)
	found := false
	for _, r := range refs {
		if r.Reason == "inject-mode experiment is not declared reversible" {
			found = true
		}
	}
	if !found {
		t.Fatalf("non-reversible inject must be refused, got %+v", refs)
	}
}

func TestRequiredApprovalsTable(t *testing.T) {
	cases := []struct {
		env      model.Env
		count    int
		distinct bool
	}{
		{model.EnvPlayground, 0, false},
		{model.EnvStage, 1, false},
		{model.EnvPreprod, 2, false},
		{model.EnvProdCanary, 2, true},
		{model.EnvProdRing, 2, true},
	}
	for _, c := range cases {
		n, d := requiredApprovals(c.env)
		if n != c.count || d != c.distinct {
			t.Errorf("%s: got (%d,%v) want (%d,%v)", c.env, n, d, c.count, c.distinct)
		}
	}
}

func TestRequiredCapabilities(t *testing.T) {
	exp := model.Experiment{Family: model.FamilyAI, Injection: model.Injection{Mode: model.ModeInject}, Capabilities: []string{"custom-cap"}}
	got := map[string]bool{}
	for _, c := range requiredCapabilities(exp, model.EnvProdCanary) {
		got[c] = true
	}
	for _, want := range []string{"custom-cap", "prod-injection", "ai-injection"} {
		if !got[want] {
			t.Errorf("expected required capability %q, got %v", want, got)
		}
	}
	// Stage, non-AI, no declared caps → none required.
	if n := len(requiredCapabilities(model.Experiment{Family: model.FamilyMesh}, model.EnvStage)); n != 0 {
		t.Errorf("expected no required caps for a plain stage experiment, got %d", n)
	}
}

func TestGateFaultDomainClash(t *testing.T) {
	p := passingProviders()
	p.FaultDomains = clashFD{}
	g, _ := newGov(t, p)
	exp := stageExperiment()
	exp.BlastRadius.RequiresDistinctFaultDomains = true
	dec, _ := g.Evaluate(context.Background(), RunRequest{Experiment: exp, Run: stageRun(), Now: testNow})
	if !hasGate(dec, GateFaultDomain) {
		t.Fatalf("expected FAULT_DOMAIN refusal, got %+v", dec.Refusals)
	}
}

func TestGateConcurrencyFull(t *testing.T) {
	p := passingProviders()
	p.Concurrency = fullConc{}
	g, _ := newGov(t, p)
	dec, _ := g.Evaluate(context.Background(), RunRequest{Experiment: stageExperiment(), Run: stageRun(), Now: testNow})
	if !hasGate(dec, GateConcurrency) {
		t.Fatalf("expected CONCURRENCY refusal, got %+v", dec.Refusals)
	}
}

func TestGateBlackoutRefusesWithNextEligible(t *testing.T) {
	p := passingProviders()
	p.Blackout = StaticBlackout{Windows: []Window{{Name: "FREEZE", Start: testNow.Add(-time.Hour), End: testNow.Add(time.Hour)}}}
	g, _ := newGov(t, p)
	dec, _ := g.Evaluate(context.Background(), RunRequest{Experiment: stageExperiment(), Run: stageRun(), Now: testNow})
	for _, r := range dec.Refusals {
		if r.Gate == GateBlackout {
			if r.NextEligible == nil {
				t.Fatal("blackout refusal should carry a next-eligible time")
			}
			return
		}
	}
	t.Fatal("expected a BLACKOUT refusal")
}

func TestBlackoutWindowEnvFilterDoesNotApply(t *testing.T) {
	// A window scoped to prod does not apply to a stage run.
	in, _, _ := StaticBlackout{Windows: []Window{{Name: "prodfreeze", Start: testNow.Add(-time.Hour), End: testNow.Add(time.Hour), Envs: []model.Env{model.EnvProdCanary}}}}.
		InWindow(context.Background(), model.EnvStage, "checkout", testNow)
	if in {
		t.Fatal("a prod-scoped window must not apply to a stage run")
	}
}

func TestGatePromotionMissingProvider(t *testing.T) {
	p := passingProviders()
	p.Promotions = nil
	g, _ := newGov(t, p)
	run := stageRun() // stage requires a playground pass
	dec, _ := g.Evaluate(context.Background(), RunRequest{Experiment: stageExperiment(), Run: run, Now: testNow})
	if !hasGate(dec, GatePromotion) {
		t.Fatalf("missing promotion provider must refuse PROMOTION, got %+v", dec.Refusals)
	}
}

func hasGate(dec Decision, g GateID) bool {
	for _, r := range dec.Refusals {
		if r.Gate == g {
			return true
		}
	}
	return false
}

type clashFD struct{}

func (clashFD) Clash(context.Context, string) (bool, string) { return true, "run-x" }

type fullConc struct{}

func (fullConc) Slot(context.Context, RunRequest) (bool, int, int) { return false, 5, 5 }
