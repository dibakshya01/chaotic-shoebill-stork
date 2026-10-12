// Package demo wires the entire platform in-process and runs a realistic matrix of
// experiments against a mock environment, so `shoebill demo` reproduces the whole
// detonate -> verify -> score -> sign loop on a laptop — including real FAIL_OPEN
// findings — with zero external dependencies.
package demo

import (
	"context"
	"path/filepath"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/detonator"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/engine"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/evidence"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/prober"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/safety"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/score"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/sim"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/store"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/verify"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/verify/sources"
)

// KeyID is the demo signing key identifier (a local throwaway; never a trusted root).
const KeyID = "shoebill-demo"

// Paths returns the standard locations under a base directory.
func StatePath(base string) string   { return filepath.Join(base, "state.json") }
func EvidenceDir(base string) string { return filepath.Join(base, "evidence") }
func KeyPath(base string) string     { return filepath.Join(base, "evidence.key") }

// Result summarizes a demo run.
type Result struct {
	Base      string
	Runs      int
	FailOpen  int
	Scorecard score.Scorecard
}

type scenario string

const (
	scPass       scenario = "pass"
	scFailOpen   scenario = "failopen"
	scUndetected scenario = "undetected"
	scSlow       scenario = "slow"
)

// Seed runs the demo and persists state + evidence under base. It uses a fresh,
// persisted-but-throwaway key under base (git-ignored), and the wall clock so the
// seeded scorecard is fresh.
func Seed(base string) (Result, error) {
	now := time.Now
	priv, err := evidence.LoadOrCreateKey(KeyPath(base))
	if err != nil {
		return Result{}, err
	}

	env := sim.New()
	env.ProgramSteady("up", sim.Response{Found: true}) // steady state holds before each injection

	reg := sources.Registry{model.SourceMock: sources.NewMock(env, model.SourceMock)}
	promo := &safety.MapPromotionHistory{}

	providers := safety.Providers{
		KillSwitch: safety.StaticKillSwitch{On: false}, Incidents: safety.NoIncidents{},
		Blackout: safety.StaticBlackout{}, Promotions: promo, Concurrency: safety.FreeConcurrency{Max: 5},
		FaultDomains: safety.NoFaultDomainClash{}, SteadyState: prober.New(reg),
	}
	gov := safety.NewGovernor(providers, KeyID, priv)
	dets := detonator.Registry{"mock": detonator.NewMock(env, gov.PublicKey(), now)}
	ledger, err := evidence.OpenLedger(EvidenceDir(base), evidence.NewSigner(KeyID, priv))
	if err != nil {
		return Result{}, err
	}
	st := store.New()
	eng := engine.New(gov, dets, verify.NewEngine(reg, now), ledger, st, engine.Options{Now: now})

	// Services and the control families each is expected to pass.
	services := demoServices()
	for _, s := range services {
		st.PutService(s)
	}
	// One representative experiment per family.
	exps := demoExperiments()
	for _, e := range exps {
		st.PutExperiment(e)
		// Seed a recent playground pass so stage runs clear the promotion gate.
		promo.Record(e.Meta.Name, model.EnvPlayground, now().Add(-48*time.Hour))
	}

	// The run plan: (service, family, scenario). Families a service expects but that
	// are absent here remain UNTESTED (scored 0 — a visible, uncomfortable state).
	plan := demoPlan()
	runs, failOpen := 0, 0
	for _, p := range plan {
		exp := exps[p.family]
		svc := serviceByID(services, p.service)
		program(env, p.scn)
		run := stageRun(exp.Meta.Name, svc)
		res, rerr := eng.Run(context.Background(), exp, run, nil)
		if rerr != nil {
			return Result{}, rerr
		}
		runs++
		if res.Verdict == model.FailOpen {
			failOpen++
		}
	}

	if err := st.Save(StatePath(base)); err != nil {
		return Result{}, err
	}
	return Result{Base: base, Runs: runs, FailOpen: failOpen, Scorecard: st.Scorecard(now())}, nil
}

// program sets the mock environment's during-injection responses for a scenario.
func program(env *sim.MockEnv, scn scenario) {
	switch scn {
	case scFailOpen:
		env.ProgramDuring("unauth_success", sim.Response{Found: true}) // control permitted it
		env.ProgramDuring("detected", sim.Response{Found: true, LatencyMs: 30000})
		env.ProgramDuring("alerted", sim.Response{Found: true, LatencyMs: 90000})
	case scUndetected:
		env.ProgramDuring("unauth_success", sim.Response{Found: false})
		env.ProgramDuring("detected", sim.Response{Found: false})
		env.ProgramDuring("alerted", sim.Response{Found: false})
	case scSlow:
		env.ProgramDuring("unauth_success", sim.Response{Found: false})
		env.ProgramDuring("detected", sim.Response{Found: true, LatencyMs: 120000}) // > 60s SLO
		env.ProgramDuring("alerted", sim.Response{Found: true, LatencyMs: 150000})
	default: // pass
		env.ProgramDuring("unauth_success", sim.Response{Found: false})
		env.ProgramDuring("detected", sim.Response{Found: true, LatencyMs: 18000})
		env.ProgramDuring("alerted", sim.Response{Found: true, LatencyMs: 90000})
	}
}

func stageRun(expName string, svc model.Service) model.Run {
	return model.Run{
		ExperimentRef: expName,
		Target: model.Target{
			Service:   svc,
			Namespace: svc.ID + "-stage", Env: model.EnvStage, Ring: model.RingCanary,
			Cluster: "stage-1", DC: "region-a",
		},
		Scope:     model.Scope{PodCount: 1, PctOfReplicas: 10, DurationSec: 45},
		Approvals: []model.Approval{{Role: "service_owner", Approver: "demo", Team: svc.Team, At: time.Now()}},
	}
}
