// Package engine runs the seven-step core loop: hypothesize -> gate -> arm ->
// detonate -> observe -> revert -> adjudicate. It is the ONLY caller of a detonator's
// Detonate, and it calls the Safety Governor immediately before — the structural "one
// door to injection".
package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/detonator"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/evidence"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/findings"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/safety"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/store"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/verify"
)

// Engine wires the loop together.
type Engine struct {
	gov        safety.Governor
	detonators detonator.Registry
	verifier   *verify.Engine
	ledger     *evidence.Ledger
	store      *store.Memory
	now        func() time.Time
	newID      func() string
	hbTimeout  time.Duration
}

// Options configure an Engine.
type Options struct {
	Now             func() time.Time
	NewID           func() string
	HeartbeatWindow time.Duration
}

// New builds an engine.
func New(gov safety.Governor, dets detonator.Registry, v *verify.Engine, ledger *evidence.Ledger, st *store.Memory, opts Options) *Engine {
	e := &Engine{gov: gov, detonators: dets, verifier: v, ledger: ledger, store: st,
		now: opts.Now, newID: opts.NewID, hbTimeout: opts.HeartbeatWindow}
	if e.now == nil {
		e.now = time.Now
	}
	if e.newID == nil {
		e.newID = func() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }
	}
	if e.hbTimeout == 0 {
		e.hbTimeout = 10 * time.Second
	}
	return e
}

// Result summarizes a run.
type Result struct {
	Decision safety.Decision
	RunID    string
	Verdict  model.Verdict
	Ladder   model.Ladder
	Receipt  *evidence.Receipt
	Finding  *findings.Finding
	Reverted bool
}

// Plan evaluates only the Safety Governor and returns the decision WITHOUT detonating.
// It backs the "safety preview" screen and `shoebill run plan`.
func (e *Engine) Plan(ctx context.Context, exp model.Experiment, run model.Run, grantedCaps []string) (safety.Decision, error) {
	return e.gov.Evaluate(ctx, safety.RunRequest{Experiment: exp, Run: run, GrantedCapabilities: grantedCaps, Now: e.now()})
}

// Run executes the full loop for one experiment against one target.
func (e *Engine) Run(ctx context.Context, exp model.Experiment, run model.Run, grantedCaps []string) (Result, error) {
	// 2. GATE — the only authorization to inject.
	dec, err := e.gov.Evaluate(ctx, safety.RunRequest{Experiment: exp, Run: run, GrantedCapabilities: grantedCaps, Now: e.now()})
	if err != nil {
		return Result{}, fmt.Errorf("safety governor: %w", err)
	}
	if !dec.Allowed {
		reasons := make([]string, 0, len(dec.Refusals))
		for _, r := range dec.Refusals {
			reasons = append(reasons, fmt.Sprintf("[%s] %s", r.Gate, r.Reason))
		}
		e.store.AddRun(store.RunView{
			ID: "r-" + e.newID(), Experiment: exp.Meta.Name, Title: exp.Meta.Title,
			Service: run.Target.Service, Family: exp.Family, Env: run.Target.Env, Ring: run.Target.Ring,
			At: e.now(), Refused: true, Refusals: reasons,
		})
		return Result{Decision: dec}, nil
	}

	cap := *dec.Capability
	runID := "r-" + e.newID()
	spec := detonator.Spec{
		Target: safety.CanonicalTarget(run.Target),
		Action: actionOf(exp),
		Params: exp.Injection.Spec,
	}
	det := e.detonators.Get(exp.Injection.Detonator)
	if det == nil {
		return Result{}, fmt.Errorf("no detonator registered for kind %q", exp.Injection.Detonator)
	}

	started := e.now()
	timeline := []evidence.Event{
		{TS: started, Phase: model.PhaseArmed, Detail: "capability " + cap.ID + " issued"},
	}

	// 4. DETONATE — the single guarded call.
	aborted := false
	abortReason := ""
	if _, derr := det.Detonate(ctx, spec, cap); derr != nil {
		aborted = true
		abortReason = derr.Error()
		timeline = append(timeline, evidence.Event{TS: e.now(), Phase: model.PhaseAborted, Detail: abortReason})
	} else {
		timeline = append(timeline, evidence.Event{TS: e.now(), Phase: model.PhaseInjected, Detail: spec.Action})
	}

	// 5. OBSERVE.
	ladder, verdict, failOpenObserved := e.verifier.Verify(ctx, exp, cap.RunUUID, aborted)
	timeline = append(timeline, evidence.Event{TS: e.now(), Phase: model.PhaseObserving})

	// 6. REVERT — idempotent, independently verified.
	reverted := true
	if rerr := det.Revert(ctx, spec, cap); rerr == nil {
		if ok, _ := det.VerifyReverted(ctx, spec); !ok {
			reverted = false
		}
	} else {
		reverted = false
	}
	timeline = append(timeline, evidence.Event{TS: e.now(), Phase: model.PhaseReverting,
		Detail: fmt.Sprintf("reverted=%v", reverted)})

	// 7. ADJUDICATE -> receipt -> score -> finding.
	receipt := e.buildReceipt(runID, exp, run, dec, ladder, verdict, timeline)
	if err := e.ledger.Append(receipt); err != nil {
		return Result{}, fmt.Errorf("append evidence: %w", err)
	}

	raw := ladder.RawScore()
	e.store.AddRun(store.RunView{
		ID: runID, Experiment: exp.Meta.Name, Title: exp.Meta.Title,
		Service: run.Target.Service, Family: exp.Family, Env: run.Target.Env, Ring: run.Target.Ring,
		Verdict: verdict, Raw: raw, Ladder: ladder, At: started, ReceiptID: receipt.ReceiptID,
	})

	var finding *findings.Finding
	if verdict.IsFailure() || failOpenObserved {
		f := e.buildFinding(exp, run, verdict, failOpenObserved, ladder, receipt.ReceiptID, started)
		e.store.UpsertFinding(f)
		finding = &f
	}

	return Result{
		Decision: dec, RunID: runID, Verdict: verdict, Ladder: ladder,
		Receipt: receipt, Finding: finding, Reverted: reverted,
	}, nil
}

func (e *Engine) buildReceipt(runID string, exp model.Experiment, run model.Run, dec safety.Decision, ladder model.Ladder, verdict model.Verdict, timeline []evidence.Event) *evidence.Receipt {
	refusals := make([]string, 0, len(dec.Refusals))
	for _, r := range dec.Refusals {
		refusals = append(refusals, string(r.Gate)+": "+r.Reason)
	}
	var probes []evidence.ProbeCheck
	for _, p := range dec.Preflight.Probes {
		probes = append(probes, evidence.ProbeCheck{Query: p.Query, Expected: p.Expected, Result: p.Result, OK: p.OK})
	}
	var capRef *evidence.Capability
	if dec.Capability != nil {
		capRef = &evidence.Capability{
			ID: dec.Capability.ID, Scopes: dec.Capability.Scopes,
			IssuedAt: dec.Capability.IssuedAt, ExpiresAt: dec.Capability.ExpiresAt,
		}
	}
	return &evidence.Receipt{
		ReceiptID:     "rc-" + e.newID(),
		SchemaVersion: evidence.SchemaVersion,
		Run: evidence.ReceiptRun{
			ID: runID, UUID: capUUID(dec), DefinitionID: exp.Meta.Name, DefinitionVersion: exp.Meta.Version,
			ManifestHash: manifestHash(exp), Env: run.Target.Env, Ring: run.Target.Ring,
			DC: run.Target.DC, Cluster: run.Target.Cluster, Namespace: run.Target.Namespace,
			Service: run.Target.Service,
		},
		Governance: evidence.Governance{
			PolicyVersion:   dec.PolicyVersion,
			SafetyDecision:  evidence.SafetyDecision{Allowed: dec.Allowed, Refusals: refusals, GatesEvaluated: dec.GatesEvaluated},
			Approvals:       run.Approvals,
			Capability:      capRef,
			BlackoutChecked: true,
		},
		Preflight:       evidence.Preflight{SteadyStateHeld: dec.Preflight.Held, Probes: probes},
		Timeline:        timeline,
		Ladder:          ladder,
		Verdict:         verdict,
		ControlFamilies: exp.ControlFamilies,
		AttackIDs:       exp.AttackIDs,
		OwaspIDs:        exp.OwaspIDs,
	}
}

func (e *Engine) buildFinding(exp model.Experiment, run model.Run, verdict model.Verdict, failOpen bool, ladder model.Ladder, receiptID string, at time.Time) findings.Finding {
	v := verdict
	if failOpen {
		v = model.FailOpen // a fail-open observation defines the finding even under an abort
	}
	detail := exp.Description
	for _, s := range ladder {
		if s.FailOpen || s.Outcome == model.StageFail {
			detail = s.Detail
			break
		}
	}
	return findings.Finding{
		ID:        "f-" + e.newID(),
		Verdict:   v,
		Severity:  findings.SeverityFor(v, run.Target.Service.Tier),
		ServiceID: run.Target.Service.ID, ServiceName: run.Target.Service.Name,
		Tier: run.Target.Service.Tier, Family: exp.Family, Experiment: exp.Meta.Name,
		Title:           titleFor(exp, v),
		Detail:          detail,
		AttackIDs:       exp.AttackIDs,
		OwaspIDs:        exp.OwaspIDs,
		ControlFamilies: exp.ControlFamilies,
		Team:            run.Target.Service.Team,
		OpenedAt:        at, UpdatedAt: at, SeenCount: 1,
		Status: findings.Open, ReceiptID: receiptID,
	}
}

func titleFor(exp model.Experiment, v model.Verdict) string {
	t := exp.Meta.Title
	if t == "" {
		t = exp.Meta.Name
	}
	switch v {
	case model.FailOpen:
		return t + " — control failed open"
	case model.FailUndetected:
		return t + " — condition went undetected"
	case model.FailSlow:
		return t + " — detection outside SLO"
	default:
		return t
	}
}

func actionOf(exp model.Experiment) string {
	if a, ok := exp.Injection.Spec["action"].(string); ok {
		return a
	}
	return exp.Injection.Detonator
}

func capUUID(dec safety.Decision) string {
	if dec.Capability != nil {
		return dec.Capability.RunUUID
	}
	return ""
}

func manifestHash(exp model.Experiment) string {
	b, _ := json.Marshal(exp)
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
