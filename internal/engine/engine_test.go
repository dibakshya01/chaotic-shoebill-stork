package engine

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/detonator"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/evidence"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/prober"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/safety"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/sim"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/store"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/verify"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/verify/sources"
)

var tnow = time.Unix(1_700_000_000, 0).UTC()

func clk() time.Time { return tnow }

func ids() func() string {
	n := 0
	return func() string { n++; return itoa(n) }
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

func failOpenExperiment() model.Experiment {
	rf := false
	return model.Experiment{
		Meta:        model.ExperimentMeta{Name: "fo-exp", Version: 1, Title: "Auth under dependency failure"},
		Family:      model.FamilyGateway,
		Description: "When the session service is degraded, requests must fail closed.",
		AttackIDs:   []string{"T1078"}, OwaspIDs: []string{"API2:2023"}, ControlFamilies: []string{"PCI-DSS-8.2"},
		Injection: model.Injection{
			Detonator: "mock", Mode: model.ModeInject, Spec: map[string]any{"action": "degrade-session"},
			Duration: model.Duration(45 * time.Second), Reversible: true,
		},
		Hypothesis: model.Hypothesis{SteadyState: []model.Probe{
			{Source: model.SourceMock, Query: "up", Expect: model.Expectation{Type: model.ExpectPresent}},
		}},
		Assertions: []model.Assertion{
			{Stage: model.Prevented, Source: model.SourceMock, Query: "unauth_success",
				Expect: model.Expectation{Type: model.ExpectAbsent}, SecurityRelevant: true, RequireUUID: &rf},
		},
		BlastRadius: model.BlastRadius{MaxPods: 1, MaxPctReplicas: 20, MaxDurationSeconds: 60},
	}
}

func stageRun() model.Run {
	return model.Run{
		ExperimentRef: "fo-exp",
		Target: model.Target{
			Service:   model.Service{ID: "checkout", Name: "checkout", Tier: 2, Team: "checkout"},
			Namespace: "checkout-stage", Env: model.EnvStage, Ring: model.RingCanary,
		},
		Scope:     model.Scope{PodCount: 1, PctOfReplicas: 10, DurationSec: 45},
		Approvals: []model.Approval{{Role: "service_owner", Team: "checkout", At: tnow}},
	}
}

func wire(t *testing.T, env *sim.MockEnv, killOn bool) (*Engine, string, ed25519.PublicKey) {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(nil)
	reg := sources.Registry{model.SourceMock: sources.NewMock(env, model.SourceMock)}
	promo := &safety.MapPromotionHistory{}
	promo.Record("fo-exp", model.EnvPlayground, tnow.Add(-24*time.Hour))
	providers := safety.Providers{
		KillSwitch: safety.StaticKillSwitch{On: killOn}, Incidents: safety.NoIncidents{},
		Blackout: safety.StaticBlackout{}, Promotions: promo, Concurrency: safety.FreeConcurrency{Max: 5},
		FaultDomains: safety.NoFaultDomainClash{}, SteadyState: prober.New(reg),
	}
	gov := safety.NewGovernor(providers, "k", priv, safety.WithClock(clk), safety.WithIDGen(ids()))
	dets := detonator.Registry{"mock": detonator.NewMock(env, gov.PublicKey(), clk)}
	dir := filepath.Join(t.TempDir(), "ev")
	ledger, err := evidence.OpenLedger(dir, evidence.NewSigner("k", priv, evidence.WithClock(clk)))
	if err != nil {
		t.Fatal(err)
	}
	eng := New(gov, dets, verify.NewEngine(reg, clk), ledger, store.New(),
		Options{Now: clk, NewID: ids()})
	return eng, dir, pub
}

func TestEngineFullLoopProducesSignedFailOpen(t *testing.T) {
	env := sim.New()
	env.ProgramSteady("up", sim.Response{Found: true})             // steady state holds pre-injection
	env.ProgramDuring("unauth_success", sim.Response{Found: true}) // control serves unauth -> FAIL_OPEN

	eng, dir, pub := wire(t, env, false)
	res, err := eng.Run(context.Background(), failOpenExperiment(), stageRun(), nil)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Verdict != model.FailOpen {
		t.Fatalf("verdict = %s, want FAIL_OPEN", res.Verdict)
	}
	if res.Finding == nil {
		t.Fatal("a fail-open run must open a finding")
	}
	if !res.Reverted {
		t.Fatal("the injected condition must be reverted")
	}
	if !env.Reverted() {
		t.Fatal("mock environment should be reverted after the run")
	}

	// The receipt is signed and the ledger verifies.
	rep, _ := evidence.VerifyLedger(dir, evidence.NewVerifier(map[string]ed25519.PublicKey{"k": pub}))
	if !rep.OK || rep.Count != 1 {
		t.Fatalf("ledger should hold 1 valid receipt, got ok=%v count=%d errs=%v", rep.OK, rep.Count, rep.Errors)
	}
}

func TestEngineRefusesWithoutDetonating(t *testing.T) {
	// Kill switch engaged -> governor denies -> the engine must NOT detonate.
	env := sim.New()
	env.ProgramSteady("up", sim.Response{Found: true})
	eng, dir, pub := wire(t, env, true /*killOn*/)
	res, err := eng.Run(context.Background(), failOpenExperiment(), stageRun(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision.Allowed {
		t.Fatal("expected refusal")
	}
	if env.Active() {
		t.Fatal("ONE DOOR VIOLATION: the engine detonated despite a governor refusal")
	}
	// No receipt is written on refusal.
	rep, _ := evidence.VerifyLedger(dir, evidence.NewVerifier(map[string]ed25519.PublicKey{"k": pub}))
	if rep.Count != 0 {
		t.Fatalf("no receipt should be written on refusal, got %d", rep.Count)
	}
}

func TestEngineArmsDeadManAndRegistersSuppression(t *testing.T) {
	env := sim.New()
	env.ProgramSteady("up", sim.Response{Found: true})
	env.ProgramDuring("unauth_success", sim.Response{Found: false}) // control holds
	exp := failOpenExperiment()
	exp.ExpectedNoise = []string{"ProxyRestartRate"} // not an asserted alert rule
	eng, _, _ := wire(t, env, false)
	res, err := eng.Run(context.Background(), exp, stageRun(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// The engine must ARM the dead-man's switch and record it (C1 fix).
	armed := false
	for _, ev := range res.Receipt.Timeline {
		if strings.Contains(ev.Detail, "dead-man's switch armed") {
			armed = true
		}
	}
	if !armed {
		t.Fatal("engine must arm the dead-man's switch and record it in the receipt timeline")
	}
	// The engine must REGISTER the expected-noise suppression (C1 fix), capped at 30m.
	sup := res.Receipt.Governance.Suppressions
	if len(sup) != 1 || sup[0].Rule != "ProxyRestartRate" {
		t.Fatalf("expected the expected-noise suppression recorded, got %+v", sup)
	}
	if sup[0].TTLSeconds > int((30 * time.Minute).Seconds()) {
		t.Fatalf("suppression TTL must be capped at 30m, got %ds", sup[0].TTLSeconds)
	}
}

// flakyRevert fails its first N Revert calls, to exercise the dead-man's-switch
// forced-revert path.
type flakyRevert struct {
	*detonator.Mock
	failUntil int
	calls     int
}

func (f *flakyRevert) Revert(ctx context.Context, spec detonator.Spec, cap model.Capability) error {
	f.calls++
	if f.calls <= f.failUntil {
		return fmt.Errorf("transient revert failure #%d", f.calls)
	}
	return f.Mock.Revert(ctx, spec, cap)
}

func TestEngineDeadManForcesRevertWhenRevertFails(t *testing.T) {
	env := sim.New()
	env.ProgramSteady("up", sim.Response{Found: true})
	env.ProgramDuring("unauth_success", sim.Response{Found: false})

	_, priv, _ := ed25519.GenerateKey(nil)
	reg := sources.Registry{model.SourceMock: sources.NewMock(env, model.SourceMock)}
	promo := &safety.MapPromotionHistory{}
	promo.Record("fo-exp", model.EnvPlayground, tnow.Add(-24*time.Hour))
	providers := safety.Providers{
		KillSwitch: safety.StaticKillSwitch{}, Incidents: safety.NoIncidents{}, Blackout: safety.StaticBlackout{},
		Promotions: promo, Concurrency: safety.FreeConcurrency{Max: 5}, FaultDomains: safety.NoFaultDomainClash{},
		SteadyState: prober.New(reg),
	}
	gov := safety.NewGovernor(providers, "k", priv, safety.WithClock(clk), safety.WithIDGen(ids()))
	// Register a detonator whose first Revert fails.
	flaky := &flakyRevert{Mock: detonator.NewMock(env, gov.PublicKey(), clk), failUntil: 1}
	dets := detonator.Registry{"mock": flaky}
	ledger, _ := evidence.OpenLedger(filepath.Join(t.TempDir(), "ev"), evidence.NewSigner("k", priv, evidence.WithClock(clk)))
	eng := New(gov, dets, verify.NewEngine(reg, clk), ledger, store.New(), Options{Now: clk, NewID: ids()})

	res, err := eng.Run(context.Background(), failOpenExperiment(), stageRun(), nil)
	if err != nil {
		t.Fatal(err)
	}
	// Our own revert failed...
	if res.Reverted {
		t.Fatal("precondition: the first revert should have failed")
	}
	// ...but the dead-man's switch fired a forced revert, so the fault is undone anyway.
	if !env.Reverted() {
		t.Fatal("the dead-man's switch must force a revert when our revert fails")
	}
	fired := false
	for _, ev := range res.Receipt.Timeline {
		if strings.Contains(ev.Detail, "dead-man's switch fired") {
			fired = true
		}
	}
	if !fired {
		t.Fatal("the forced revert must be recorded in the receipt")
	}
	if flaky.calls < 2 {
		t.Fatalf("expected the dead-man's switch to retry the revert, calls=%d", flaky.calls)
	}
}

func TestEnginePassWhenControlHolds(t *testing.T) {
	env := sim.New()
	env.ProgramSteady("up", sim.Response{Found: true})
	env.ProgramDuring("unauth_success", sim.Response{Found: false}) // control holds -> PREVENTED pass
	eng, _, _ := wire(t, env, false)
	res, err := eng.Run(context.Background(), failOpenExperiment(), stageRun(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != model.Pass {
		t.Fatalf("verdict = %s, want PASS", res.Verdict)
	}
	if res.Finding != nil {
		t.Fatal("a passing run must not open a finding")
	}
}
