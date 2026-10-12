package verify

import (
	"context"
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/sim"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/verify/sources"
)

func boolp(b bool) *bool { return &b }

// experiment with a PREVENTED (security-relevant absent), DETECTED and ALERTED ladder.
func ladderExperiment() model.Experiment {
	return model.Experiment{
		Meta: model.ExperimentMeta{Name: "mesh-06", Version: 1},
		Assertions: []model.Assertion{
			{Stage: model.Prevented, Source: model.SourceMock, Query: "unauth_success",
				Expect: model.Expectation{Type: model.ExpectAbsent}, SecurityRelevant: true, RequireUUID: boolp(false)},
			{Stage: model.Detected, Source: model.SourceMock, Query: "detected",
				Expect: model.Expectation{Type: model.ExpectPresent}, SLOms: 60000},
			{Stage: model.Alerted, Source: model.SourceMock, Query: "alerted",
				Expect: model.Expectation{Type: model.ExpectPresent}, SLOms: 300000},
		},
	}
}

func engineOver(env *sim.MockEnv) *Engine {
	reg := sources.Registry{model.SourceMock: sources.NewMock(env, model.SourceMock)}
	return NewEngine(reg, func() time.Time { return time.Unix(1_700_000_000, 0) })
}

func TestVerifyScenarios(t *testing.T) {
	const uuid = "u1"
	cases := []struct {
		name string
		prog func(e *sim.MockEnv)
		want model.Verdict
	}{
		{
			name: "healthy -> PASS",
			prog: func(e *sim.MockEnv) {
				e.ProgramDuring("unauth_success", sim.Response{Found: false})
				e.ProgramDuring("detected", sim.Response{Found: true, LatencyMs: 20000})
				e.ProgramDuring("alerted", sim.Response{Found: true, LatencyMs: 120000})
			},
			want: model.Pass,
		},
		{
			name: "control serves unauthenticated -> FAIL_OPEN",
			prog: func(e *sim.MockEnv) {
				e.ProgramDuring("unauth_success", sim.Response{Found: true}) // the forbidden thing happened
				e.ProgramDuring("detected", sim.Response{Found: true})
				e.ProgramDuring("alerted", sim.Response{Found: true})
			},
			want: model.FailOpen,
		},
		{
			name: "prevented but detection decayed -> FAIL_UNDETECTED (alert chain-skipped)",
			prog: func(e *sim.MockEnv) {
				e.ProgramDuring("unauth_success", sim.Response{Found: false})
				e.ProgramDuring("detected", sim.Response{Found: false})
				// alerted would error, but must be SKIPPED, not INCONCLUSIVE:
				e.ProgramDuring("alerted", sim.Response{Err: true})
			},
			want: model.FailUndetected,
		},
		{
			name: "detection slow -> FAIL_SLOW",
			prog: func(e *sim.MockEnv) {
				e.ProgramDuring("unauth_success", sim.Response{Found: false})
				e.ProgramDuring("detected", sim.Response{Found: true, LatencyMs: 120000}) // > 60s SLO
				e.ProgramDuring("alerted", sim.Response{Found: true, LatencyMs: 120000})
			},
			want: model.FailSlow,
		},
		{
			name: "detection source unavailable -> INCONCLUSIVE",
			prog: func(e *sim.MockEnv) {
				e.ProgramDuring("unauth_success", sim.Response{Found: false})
				e.ProgramDuring("detected", sim.Response{Err: true})
				e.ProgramDuring("alerted", sim.Response{Found: true})
			},
			want: model.Inconclusive,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := sim.New()
			tc.prog(env)
			env.Inject(uuid)
			_, verdict, _ := engineOver(env).Verify(context.Background(), ladderExperiment(), uuid, false)
			if verdict != tc.want {
				t.Fatalf("verdict = %s, want %s", verdict, tc.want)
			}
		})
	}
}

func TestVerifyUUIDCorrelationRequired(t *testing.T) {
	// The detected signal is present, but for a DIFFERENT run's uuid: it must not count.
	env := sim.New()
	env.ProgramDuring("unauth_success", sim.Response{Found: false})
	env.ProgramDuring("detected", sim.Response{Found: true})
	env.ProgramDuring("alerted", sim.Response{Found: true})
	env.Inject("the-real-run")
	// Verify with a different uuid: MatchedUUID will be false, so present assertions miss.
	_, verdict, _ := engineOver(env).Verify(context.Background(), ladderExperiment(), "some-other-run", false)
	if verdict != model.FailUndetected {
		t.Fatalf("uncorrelated signal must not score a PASS; got %s", verdict)
	}
}
