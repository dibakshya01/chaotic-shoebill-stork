// Package verify is the verification engine: it queries signal sources to observe a
// control's response across the ladder, correlates on the run UUID, and adjudicates a
// single honest verdict. A source that errors yields INCONCLUSIVE, never PASS.
package verify

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/verify/sources"
)

// Engine evaluates assertions against configured signal sources.
type Engine struct {
	reg sources.Registry
	now func() time.Time
}

// NewEngine builds a verification engine over a source registry.
func NewEngine(reg sources.Registry, now func() time.Time) *Engine {
	if now == nil {
		now = time.Now
	}
	return &Engine{reg: reg, now: now}
}

var ladderRank = func() map[model.LadderStage]int {
	m := map[model.LadderStage]int{}
	for i, s := range model.LadderOrder {
		m[s] = i
	}
	return m
}()

// breaksChain reports whether a non-success at this stage should skip later stages in
// the detection sub-ladder (Detected -> Alerted -> Paged -> Ticketed).
func breaksChain(s model.LadderStage) bool {
	switch s {
	case model.Detected, model.Alerted, model.Paged:
		return true
	default:
		return false
	}
}

func isChainStage(s model.LadderStage) bool {
	switch s {
	case model.Alerted, model.Paged, model.Ticketed:
		return true
	default:
		return false
	}
}

// BuildLadder evaluates every assertion (in ladder order) into a ladder, applying the
// skip chain so a page/ticket assertion is not spuriously marked INCONCLUSIVE when the
// alert it depends on never fired.
func (e *Engine) BuildLadder(ctx context.Context, exp model.Experiment, uuid string) model.Ladder {
	assertions := append([]model.Assertion(nil), exp.Assertions...)
	sort.SliceStable(assertions, func(i, j int) bool {
		return ladderRank[assertions[i].Stage] < ladderRank[assertions[j].Stage]
	})

	var ladder model.Ladder
	chainBroken := false

	for _, a := range assertions {
		res := model.StageResult{Stage: a.Stage, SLOms: a.SLOms}

		if isChainStage(a.Stage) && chainBroken {
			res.Outcome = model.StageSkip
			res.Detail = "not reached: an earlier stage in the detection chain did not succeed"
			ladder = append(ladder, res)
			continue
		}

		src := e.reg.Get(a.Source)
		if src == nil {
			res.Outcome = model.StageInconclusive
			res.Detail = fmt.Sprintf("signal source %q not configured", a.Source)
			ladder = append(ladder, res)
			if breaksChain(a.Stage) {
				chainBroken = true
			}
			continue
		}

		obs, err := src.Query(ctx, a.Query, uuid)
		if err != nil {
			res.Outcome = model.StageInconclusive
			res.Detail = fmt.Sprintf("source %q unavailable: %v", a.Source, err)
			ladder = append(ladder, res)
			if breaksChain(a.Stage) {
				chainBroken = true
			}
			continue
		}

		satisfied, failOpen := evalExpectation(a, obs)
		if obs.LatencyMs > 0 {
			l := obs.LatencyMs
			res.LatencyMs = &l
		}
		switch {
		case failOpen:
			res.Outcome = model.StageFail
			res.FailOpen = true
			res.Detail = "control permitted something it should have denied (fail-open)"
		case satisfied && a.SLOms > 0 && obs.LatencyMs > a.SLOms:
			res.Outcome = model.StageSlow
			res.Detail = fmt.Sprintf("matched but at %dms > SLO %dms", obs.LatencyMs, a.SLOms)
		case satisfied:
			res.Outcome = model.StagePass
		default:
			res.Outcome = model.StageFail
			res.Detail = "expectation not satisfied"
		}
		ladder = append(ladder, res)

		if breaksChain(a.Stage) && res.Outcome != model.StagePass && res.Outcome != model.StageSlow {
			chainBroken = true
		}
	}
	return ladder
}

// Verify builds the ladder and adjudicates. failOpenObserved is reported separately so
// a permissive failure is recorded even when the run was aborted (FR-11a).
func (e *Engine) Verify(ctx context.Context, exp model.Experiment, uuid string, aborted bool) (model.Ladder, model.Verdict, bool) {
	ladder := e.BuildLadder(ctx, exp, uuid)
	verdict, failOpen := Adjudicate(ladder, aborted)
	return ladder, verdict, failOpen
}
