package safety

import (
	"context"
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

var testNow = time.Unix(1_700_000_000, 0).UTC()

func clock() func() time.Time { return func() time.Time { return testNow } }

// counterID yields deterministic ids for tests.
func counterID() func() string {
	n := 0
	return func() string { n++; return "id" + itoa(n) }
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// heldProber returns a steady-state proof that holds.
type heldProber struct{ held bool }

func (h heldProber) Probe(_ context.Context, probes []model.Probe, _ map[string]any) SteadyStateProof {
	pr := SteadyStateProof{Held: h.held}
	for _, p := range probes {
		pr.Probes = append(pr.Probes, ProbeResult{Query: p.Query, Expected: "present", Result: "present", OK: h.held})
	}
	return pr
}

func stageExperiment() model.Experiment {
	return model.Experiment{
		Meta:   model.ExperimentMeta{Name: "mesh-06", Version: 1},
		Family: model.FamilyMesh,
		Injection: model.Injection{
			Detonator:  "mock",
			Mode:       model.ModeInject,
			Spec:       map[string]any{"action": "container-kill"},
			Duration:   model.Duration(45 * time.Second),
			Reversible: true,
		},
		Hypothesis: model.Hypothesis{
			SteadyState: []model.Probe{{Source: model.SourceMock, Query: "up", Expect: model.Expectation{Type: model.ExpectPresent}}},
		},
		BlastRadius: model.BlastRadius{MaxPods: 1, MaxPctReplicas: 10, MaxDurationSeconds: 60, MaxTrafficPct: 0},
	}
}

func stageRun() model.Run {
	return model.Run{
		ExperimentRef: "mesh-06",
		Target: model.Target{
			Service:   model.Service{ID: "checkout", Name: "checkout", Tier: 2, Team: "checkout"},
			Namespace: "checkout-stage", Env: model.EnvStage, Ring: model.RingCanary,
		},
		Scope:     model.Scope{PodCount: 1, PctOfReplicas: 10, DurationSec: 45, PctTraffic: 0},
		Approvals: []model.Approval{{Role: "service_owner", Approver: "a", Team: "checkout", At: testNow}},
	}
}

func passingProviders() Providers {
	promo := &MapPromotionHistory{}
	promo.Record("mesh-06", model.EnvPlayground, testNow.Add(-24*time.Hour))
	return Providers{
		KillSwitch:   StaticKillSwitch{On: false},
		Incidents:    NoIncidents{},
		Blackout:     StaticBlackout{},
		Promotions:   promo,
		Concurrency:  FreeConcurrency{Max: 5},
		FaultDomains: NoFaultDomainClash{},
		SteadyState:  heldProber{held: true},
	}
}

func newGov(t *testing.T, p Providers) (*DefaultGovernor, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	g := NewGovernor(p, "gov-key", priv, WithClock(clock()), WithIDGen(counterID()))
	return g, pub
}

func TestGovernorAllowsCleanStageRun(t *testing.T) {
	g, pub := newGov(t, passingProviders())
	dec, err := g.Evaluate(context.Background(), RunRequest{Experiment: stageExperiment(), Run: stageRun(), Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Allowed {
		t.Fatalf("expected allowed, refusals: %+v", dec.Refusals)
	}
	if dec.GatesEvaluated != 10 {
		t.Fatalf("expected 10 gates evaluated, got %d", dec.GatesEvaluated)
	}
	if dec.Capability == nil {
		t.Fatal("expected a minted capability")
	}
	// The minted capability must verify against the governor key and authorize the action.
	target := CanonicalTarget(stageRun().Target)
	if err := dec.Capability.Authorize(pub, testNow, target, "detonator:mock"); err != nil {
		t.Fatalf("capability should authorize the detonator action: %v", err)
	}
	if err := dec.Capability.Authorize(pub, testNow, target, "action:container-kill"); err != nil {
		t.Fatalf("capability should authorize the declared action: %v", err)
	}
	if dec.PolicyVersion != PolicyVersion {
		t.Fatalf("policy version not recorded")
	}
}

func TestGovernorEvaluatesAllGatesNotShortCircuited(t *testing.T) {
	// A maximally-bad prod-canary request: kill switch on, active SEV, in blackout,
	// no promotion, no capability, no approvals, over budget, steady state not held.
	p := Providers{
		KillSwitch:   StaticKillSwitch{On: true},
		Incidents:    StaticIncidents{ActiveByTeam: map[string]string{"checkout": "SEV-0-123"}},
		Blackout:     StaticBlackout{Windows: []Window{{Name: "FREEZE", Start: testNow.Add(-time.Hour), End: testNow.Add(time.Hour)}}},
		Promotions:   &MapPromotionHistory{},
		Concurrency:  FreeConcurrency{Max: 5},
		FaultDomains: NoFaultDomainClash{},
		SteadyState:  heldProber{held: false},
	}
	g, _ := newGov(t, p)
	exp := stageExperiment()
	run := stageRun()
	run.Target.Env = model.EnvProdCanary
	run.Target.Service.Tier = 1
	run.Approvals = nil
	run.Scope.DurationSec = 600 // way over the 60s prod-canary budget
	run.Scope.PctTraffic = 50   // over 1%

	dec, err := g.Evaluate(context.Background(), RunRequest{Experiment: exp, Run: run, Now: testNow})
	if err != nil {
		t.Fatal(err)
	}
	if dec.Allowed {
		t.Fatal("expected refusal")
	}
	gates := map[GateID]bool{}
	for _, r := range dec.Refusals {
		gates[r.Gate] = true
	}
	// Prove gates were NOT short-circuited: refusals from many distinct gates.
	for _, want := range []GateID{GateKillSwitch, GateSEVActive, GateBlackout, GatePromotion, GateCapability, GateApproval, GateBudget, GateSteadyState} {
		if !gates[want] {
			t.Errorf("expected a refusal from gate %s; got gates %v", want, gates)
		}
	}
	if dec.Capability != nil {
		t.Fatal("no capability must be minted on refusal")
	}
}

func TestGateBudgetProdCanaryEnvelope(t *testing.T) {
	g, _ := newGov(t, passingProviders())
	exp := stageExperiment()
	run := stageRun()
	run.Target.Env = model.EnvProdCanary
	run.Target.Service.Tier = 1
	run.Scope.DurationSec = 90 // exceeds 60s
	run.Approvals = []model.Approval{
		{Team: "checkout"}, {Team: "appsec"},
	}
	dec, _ := g.Evaluate(context.Background(), RunRequest{
		Experiment: exp, Run: run, Now: testNow,
		GrantedCapabilities: []string{"prod-injection"},
	})
	if dec.Allowed {
		t.Fatal("expected budget refusal for 90s in prod-canary")
	}
	found := false
	for _, r := range dec.Refusals {
		if r.Gate == GateBudget {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a BUDGET refusal, got %+v", dec.Refusals)
	}
	// Constraints must report the tightened 60s envelope.
	if dec.Constraints.MaxDurationSeconds != 60 {
		t.Fatalf("expected max duration 60, got %d", dec.Constraints.MaxDurationSeconds)
	}
}

func TestPaymentsPathProdInjectRefused(t *testing.T) {
	g, _ := newGov(t, passingProviders())
	exp := stageExperiment() // inject mode
	run := stageRun()
	run.Target.Env = model.EnvProdCanary
	run.Target.Service.Tier = 1
	run.Target.Service.OnPaymentsPath = true
	run.Scope.DurationSec = 60
	run.Scope.PctTraffic = 1
	run.Approvals = []model.Approval{{Team: "checkout"}, {Team: "appsec"}}
	dec, _ := g.Evaluate(context.Background(), RunRequest{
		Experiment: exp, Run: run, Now: testNow, GrantedCapabilities: []string{"prod-injection"},
	})
	if dec.Allowed {
		t.Fatal("payments-path inject in prod must be refused")
	}
}

func TestSuppressionCannotCoverAssertion(t *testing.T) {
	exp := model.Experiment{
		Assertions: []model.Assertion{
			{Stage: model.Alerted, Source: model.SourceAlertmanager, Query: "SidecarUnhealthy"},
		},
		ExpectedNoise: []string{"ProxyRestartRate"},
	}
	// Suppressing the experiment's own asserted alert must be refused.
	err := ValidateSuppression(SuppressionRequest{AlertRules: []string{"SidecarUnhealthy"}, TTL: time.Minute}, exp)
	if !errors.Is(err, ErrSuppressAssertion) {
		t.Fatalf("expected ErrSuppressAssertion, got %v", err)
	}
	// Suppressing legitimate expected noise is fine.
	if err := ValidateSuppression(SuppressionRequest{AlertRules: []string{"ProxyRestartRate"}, TTL: time.Minute}, exp); err != nil {
		t.Fatalf("expected expected-noise suppression to be allowed, got %v", err)
	}
	// Wildcards are forbidden.
	if err := ValidateSuppression(SuppressionRequest{AlertRules: []string{"*"}, TTL: time.Minute}, exp); !errors.Is(err, ErrSuppressWildcard) {
		t.Fatalf("expected ErrSuppressWildcard, got %v", err)
	}
	// TTL over 30m is forbidden.
	if err := ValidateSuppression(SuppressionRequest{AlertRules: []string{"ProxyRestartRate"}, TTL: 31 * time.Minute}, exp); !errors.Is(err, ErrSuppressTTL) {
		t.Fatalf("expected ErrSuppressTTL, got %v", err)
	}
}

func TestSteadyStateNotHeldRefuses(t *testing.T) {
	p := passingProviders()
	p.SteadyState = heldProber{held: false}
	g, _ := newGov(t, p)
	dec, _ := g.Evaluate(context.Background(), RunRequest{Experiment: stageExperiment(), Run: stageRun(), Now: testNow})
	if dec.Allowed {
		t.Fatal("expected refusal when steady state does not hold")
	}
	if dec.Preflight.Held {
		t.Fatal("preflight proof should report not held")
	}
}
