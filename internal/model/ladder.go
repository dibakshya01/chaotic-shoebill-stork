package model

// LadderStage is one rung of the control-response ladder. An experiment declares
// which stages it asserts and the SLO for each. The five rungs model the full
// lifecycle of a control's response to an adversarial condition.
type LadderStage string

const (
	// Prevented: the control blocked the condition outright (the strongest outcome).
	Prevented LadderStage = "PREVENTED"
	// Detected: telemetry recorded that the condition occurred.
	Detected LadderStage = "DETECTED"
	// Alerted: an alert rule fired on the condition.
	Alerted LadderStage = "ALERTED"
	// Paged: a human was notified.
	Paged LadderStage = "PAGED"
	// Ticketed: the condition was tracked to resolution.
	Ticketed LadderStage = "TICKETED"
)

// LadderOrder is the canonical ordering of the ladder, strongest first.
var LadderOrder = []LadderStage{Prevented, Detected, Alerted, Paged, Ticketed}

// StageWeights weight each stage when computing the raw efficacy score. A control
// that prevents is worth far more than one that merely tickets after the fact.
// These sum to 1.0 over the full ladder; the raw score normalizes by the weights
// of the stages an experiment actually asserts.
var StageWeights = map[LadderStage]float64{
	Prevented: 0.40,
	Detected:  0.25,
	Alerted:   0.20,
	Paged:     0.10,
	Ticketed:  0.05,
}

// StageOutcome is the result of evaluating a single asserted stage.
type StageOutcome string

const (
	StagePass         StageOutcome = "PASS"         // matched within SLO
	StageFail         StageOutcome = "FAIL"         // did not match (or matched an "absent" assertion)
	StageSlow         StageOutcome = "SLOW"         // matched, but outside SLO
	StageSkip         StageOutcome = "SKIP"         // not reached (an earlier required stage failed)
	StageInconclusive StageOutcome = "INCONCLUSIVE" // the signal source was unavailable/errored
)

// StageResult records the outcome of one asserted ladder stage.
type StageResult struct {
	Stage     LadderStage  `json:"stage" yaml:"stage"`
	Outcome   StageOutcome `json:"outcome" yaml:"outcome"`
	LatencyMs *int64       `json:"latency_ms,omitempty" yaml:"latency_ms,omitempty"`
	SLOms     int64        `json:"slo_ms" yaml:"slo_ms"`
	Detail    string       `json:"detail,omitempty" yaml:"detail,omitempty"`
	// FailOpen marks a stage whose failure was permissive (a control allowed what
	// it should have denied). Any true value forces the run verdict to FAIL_OPEN.
	FailOpen bool `json:"fail_open,omitempty" yaml:"fail_open,omitempty"`
}

// Ladder is the ordered set of stage results for a run.
type Ladder []StageResult

// Get returns the result for a stage and whether it was asserted.
func (l Ladder) Get(s LadderStage) (StageResult, bool) {
	for _, r := range l {
		if r.Stage == s {
			return r, true
		}
	}
	return StageResult{}, false
}

// RawScore computes the weighted efficacy over the asserted stages in [0,1].
// A PASS contributes its full weight; SLOW contributes half; FAIL/SKIP/INCONCLUSIVE
// contribute zero. The result is normalized by the total weight of asserted stages,
// so an experiment that only asserts PREVENTED is still scored on [0,1].
func (l Ladder) RawScore() float64 {
	var num, den float64
	for _, r := range l {
		w := StageWeights[r.Stage]
		den += w
		switch r.Outcome {
		case StagePass:
			num += w
		case StageSlow:
			num += w * 0.5
		}
	}
	if den == 0 {
		return 0
	}
	return num / den
}
