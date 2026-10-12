package verify

import (
	"testing"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

func stage(s model.LadderStage, o model.StageOutcome) model.StageResult {
	return model.StageResult{Stage: s, Outcome: o}
}

func failOpenStage() model.StageResult {
	return model.StageResult{Stage: model.Prevented, Outcome: model.StageFail, FailOpen: true}
}

func TestAdjudicationTruthTable(t *testing.T) {
	cases := []struct {
		name         string
		ladder       model.Ladder
		aborted      bool
		wantVerdict  model.Verdict
		wantFailOpen bool
	}{
		{
			name:        "all pass -> PASS",
			ladder:      model.Ladder{stage(model.Prevented, model.StagePass), stage(model.Detected, model.StagePass)},
			wantVerdict: model.Pass,
		},
		{
			name:         "fail-open dominates an otherwise passing ladder",
			ladder:       model.Ladder{failOpenStage(), stage(model.Detected, model.StagePass), stage(model.Alerted, model.StagePass)},
			wantVerdict:  model.FailOpen,
			wantFailOpen: true,
		},
		{
			name:        "inconclusive never becomes PASS",
			ladder:      model.Ladder{stage(model.Prevented, model.StagePass), stage(model.Detected, model.StageInconclusive)},
			wantVerdict: model.Inconclusive,
		},
		{
			name:        "fail-open dominates inconclusive too",
			ladder:      model.Ladder{failOpenStage(), stage(model.Detected, model.StageInconclusive)},
			wantVerdict: model.FailOpen, wantFailOpen: true,
		},
		{
			name:        "all matched but one slow -> FAIL_SLOW",
			ladder:      model.Ladder{stage(model.Prevented, model.StagePass), stage(model.Alerted, model.StageSlow)},
			wantVerdict: model.FailSlow,
		},
		{
			name:        "a plain miss -> FAIL_UNDETECTED",
			ladder:      model.Ladder{stage(model.Prevented, model.StagePass), stage(model.Detected, model.StageFail)},
			wantVerdict: model.FailUndetected,
		},
		{
			name:        "a skip counts as a miss -> FAIL_UNDETECTED",
			ladder:      model.Ladder{stage(model.Detected, model.StageFail), stage(model.Alerted, model.StageSkip)},
			wantVerdict: model.FailUndetected,
		},
		{
			name:        "empty ladder -> INCONCLUSIVE (we learned nothing)",
			ladder:      model.Ladder{},
			wantVerdict: model.Inconclusive,
		},
		{
			name:         "aborted verdict, but fail-open still surfaced (FR-11a)",
			ladder:       model.Ladder{failOpenStage()},
			aborted:      true,
			wantVerdict:  model.Aborted,
			wantFailOpen: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, fo := Adjudicate(tc.ladder, tc.aborted)
			if v != tc.wantVerdict {
				t.Errorf("verdict = %s, want %s", v, tc.wantVerdict)
			}
			if fo != tc.wantFailOpen {
				t.Errorf("failOpenObserved = %v, want %v", fo, tc.wantFailOpen)
			}
		})
	}
}
