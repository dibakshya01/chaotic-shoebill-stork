package model

// Verdict is the adjudicated outcome of a security chaos run. The set and their
// semantics are load-bearing: see docs/safety-model.md and the adjudication rules
// in internal/verify. Two choices carry most of the platform's epistemic integrity:
//
//   - FAIL_OPEN is a distinct verdict from a plain failure. A control that degraded
//     *permissively* (allowed what it should have denied) is the worst outcome.
//   - INCONCLUSIVE must NEVER be downgraded to PASS. If we could not observe, we
//     learned nothing — reporting PASS would manufacture false confidence.
type Verdict string

const (
	// Pass: every asserted ladder stage matched within its SLO.
	Pass Verdict = "PASS"
	// FailUndetected: the condition occurred and nothing noticed it.
	FailUndetected Verdict = "FAIL_UNDETECTED"
	// FailOpen: the control permitted something it should have denied. CRITICAL.
	FailOpen Verdict = "FAIL_OPEN"
	// FailSlow: detected/alerted, but outside the stated SLO.
	FailSlow Verdict = "FAIL_SLOW"
	// Aborted: the run was stopped (kill switch, guardrail breach, operator abort).
	Aborted Verdict = "ABORTED"
	// Inconclusive: a required signal source was unavailable or ambiguous.
	Inconclusive Verdict = "INCONCLUSIVE"
)

// Valid reports whether v is a known verdict.
func (v Verdict) Valid() bool {
	switch v {
	case Pass, FailUndetected, FailOpen, FailSlow, Aborted, Inconclusive:
		return true
	default:
		return false
	}
}

// IsFailure reports whether the verdict represents a control that did not behave
// as hypothesized. INCONCLUSIVE and ABORTED are not failures of the control.
func (v Verdict) IsFailure() bool {
	switch v {
	case FailUndetected, FailOpen, FailSlow:
		return true
	default:
		return false
	}
}

// severity orders verdicts for aggregation and display (higher = worse / more
// attention-worthy). FAIL_OPEN dominates everything else a run can produce.
func (v Verdict) severity() int {
	switch v {
	case FailOpen:
		return 100
	case Aborted:
		return 70
	case Inconclusive:
		return 60
	case FailUndetected:
		return 50
	case FailSlow:
		return 40
	case Pass:
		return 10
	default:
		return 0
	}
}

// WorseThan reports whether v should take precedence over other when summarizing
// a set of runs (e.g. a service's worst current verdict for a control family).
func (v Verdict) WorseThan(other Verdict) bool {
	return v.severity() > other.severity()
}

// Label returns a short human label for UIs.
func (v Verdict) Label() string {
	switch v {
	case Pass:
		return "Pass"
	case FailUndetected:
		return "Fail — undetected"
	case FailOpen:
		return "Fail-open"
	case FailSlow:
		return "Fail — slow"
	case Aborted:
		return "Aborted"
	case Inconclusive:
		return "Inconclusive"
	default:
		return string(v)
	}
}
