package verify

import (
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/verify/sources"
)

// evalExpectation compares an observation against an assertion's expectation and
// returns whether the assertion is SATISFIED and whether a fail-open occurred (a
// security-relevant "absent" expectation that matched — the forbidden thing happened).
//
// UUID correlation: for positive expectations (present/threshold/count/value/paged) an
// observation only counts if it is correlated to this run when the assertion requires a
// UUID. For an "absent" expectation, the forbidden event counts regardless of UUID —
// fail-open is asserted over all traffic, not only synthetic traffic (this is why such
// assertions legitimately set requireUuid:false with a justification).
func evalExpectation(a model.Assertion, obs sources.Observation) (satisfied bool, failOpen bool) {
	correlated := !a.RequiresUUID() || obs.MatchedUUID
	op := a.Expect.Op

	switch a.Expect.Type {
	case model.ExpectAbsent:
		happened := obs.Found
		if happened && a.SecurityRelevant {
			return false, true // the control permitted something — FAIL_OPEN
		}
		return !happened, false

	case model.ExpectPresent:
		return obs.Found && correlated, false

	case model.ExpectPaged:
		return obs.Found && correlated, false

	case model.ExpectThreshold:
		return correlated && obs.Found && op.Compare(obs.Value, a.Expect.Value), false

	case model.ExpectCount:
		if op == "" {
			op = model.OpGTE
		}
		return correlated && op.Compare(float64(obs.Count), a.Expect.Value), false

	case model.ExpectValue:
		if op == "" {
			op = model.OpEQ
		}
		return correlated && obs.Found && op.Compare(obs.Value, a.Expect.Value), false

	default:
		return false, false
	}
}

// Adjudicate resolves a ladder (and the aborted flag) into a single verdict, following
// the first-match-wins rules in build-plan.md FR-9. It also reports whether a fail-open
// was OBSERVED, so the engine can open a FAIL_OPEN finding even when the run was aborted
// (FR-11a: an observed permissive failure is never swallowed).
func Adjudicate(ladder model.Ladder, aborted bool) (model.Verdict, bool) {
	failOpenObserved := false
	anyInconclusive := false
	anyFailOrSkip := false
	anySlow := false
	asserted := 0

	for _, s := range ladder {
		asserted++
		if s.FailOpen {
			failOpenObserved = true
		}
		switch s.Outcome {
		case model.StageInconclusive:
			anyInconclusive = true
		case model.StageFail, model.StageSkip:
			anyFailOrSkip = true
		case model.StageSlow:
			anySlow = true
		case model.StagePass:
			// ok
		}
	}

	// Rule 1: aborted dominates the VERDICT — but a fail-open observation still surfaces.
	if aborted {
		return model.Aborted, failOpenObserved
	}
	// Rule 2: any observed fail-open.
	if failOpenObserved {
		return model.FailOpen, true
	}
	// Rule 3: any asserted stage whose source was unavailable/errored.
	if anyInconclusive {
		return model.Inconclusive, false
	}
	// Nothing asserted at all → we learned nothing.
	if asserted == 0 {
		return model.Inconclusive, false
	}
	// Rule 4: everything matched within SLO.
	if !anyFailOrSkip && !anySlow {
		return model.Pass, false
	}
	// Rule 5: everything matched but something was slow.
	if !anyFailOrSkip && anySlow {
		return model.FailSlow, false
	}
	// Rule 6: at least one asserted stage did not match.
	return model.FailUndetected, false
}
