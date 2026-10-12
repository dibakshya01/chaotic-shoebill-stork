package score

import (
	"math"
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

var now = time.Unix(1_700_000_000, 0).UTC()

func TestFreshnessDecay(t *testing.T) {
	// At exactly τ days old, freshness = e^-1 ≈ 0.3679 for the matching tier.
	if got := Freshness(14, 1); math.Abs(got-math.Exp(-1)) > 1e-9 {
		t.Errorf("tier-1 freshness at 14d = %v, want e^-1", got)
	}
	if got := Freshness(30, 2); math.Abs(got-math.Exp(-1)) > 1e-9 {
		t.Errorf("tier-2 freshness at 30d = %v, want e^-1", got)
	}
	if Freshness(0, 1) != 1 {
		t.Errorf("fresh run should have freshness 1.0")
	}
}

func TestUntestedScoresZeroAndCoverage(t *testing.T) {
	svc := model.Service{ID: "checkout", Name: "checkout", Tier: 1}
	specs := []ServiceSpec{{Service: svc, Expected: []model.Family{model.FamilyMesh, model.FamilyGateway}}}
	runs := []RunRecord{
		{ServiceID: "checkout", Family: model.FamilyMesh, Experiment: "mesh-01", Verdict: model.Pass, Raw: 1.0, At: now},
	}
	sc := Build(specs, runs, nil, now)
	card := sc.Services[0]
	if card.Cells[model.FamilyGateway].Tested {
		t.Fatal("gateway was never tested; must be untested")
	}
	if card.Cells[model.FamilyGateway].Score != 0 {
		t.Fatalf("untested control must score 0, got %v", card.Cells[model.FamilyGateway].Score)
	}
	// mesh fresh PASS raw 1.0 → ~1.0; gateway 0 → service ≈ 0.5
	if math.Abs(card.Score-0.5) > 1e-6 {
		t.Fatalf("service score = %v, want 0.5", card.Score)
	}
	if sc.Summary.CoverageTested != 1 || sc.Summary.CoverageTotal != 2 {
		t.Fatalf("coverage = %d/%d, want 1/2", sc.Summary.CoverageTested, sc.Summary.CoverageTotal)
	}
}

func TestInconclusiveDoesNotInflateScore(t *testing.T) {
	svc := model.Service{ID: "checkout", Name: "checkout", Tier: 1}
	specs := []ServiceSpec{{Service: svc, Expected: []model.Family{model.FamilyMesh, model.FamilyGateway}}}
	runs := []RunRecord{
		{ServiceID: "checkout", Family: model.FamilyMesh, Verdict: model.Pass, Raw: 1.0, At: now},
		// A partially-inconclusive run: some stages passed (Raw>0) but the verdict is INCONCLUSIVE.
		{ServiceID: "checkout", Family: model.FamilyGateway, Verdict: model.Inconclusive, Raw: 0.62, At: now},
	}
	sc := Build(specs, runs, nil, now)
	card := sc.Services[0]
	gw := card.Cells[model.FamilyGateway]
	if gw.State != "untested" {
		t.Fatalf("inconclusive cell should render untested, got %s", gw.State)
	}
	if gw.Score != 0 {
		t.Fatalf("inconclusive cell must score 0 (we learned nothing), got %v", gw.Score)
	}
	// mesh 1.0 + gateway 0 → service 0.5, NOT inflated by the inconclusive raw.
	if card.Score != 0.5 {
		t.Fatalf("service score = %v, want 0.5 (inconclusive must not inflate it)", card.Score)
	}
	if sc.Summary.CoverageTested != 1 {
		t.Fatalf("inconclusive must not count as covered; coverage tested = %d", sc.Summary.CoverageTested)
	}
}

func TestTier1FailOpenCaps(t *testing.T) {
	svc := model.Service{ID: "checkout", Name: "checkout", Tier: 1}
	specs := []ServiceSpec{{Service: svc, Expected: []model.Family{model.FamilyMesh, model.FamilyGateway}}}
	// Both families pass with perfect fresh scores → would be ~1.0...
	runs := []RunRecord{
		{ServiceID: "checkout", Family: model.FamilyMesh, Verdict: model.Pass, Raw: 1.0, At: now},
		{ServiceID: "checkout", Family: model.FamilyGateway, Verdict: model.Pass, Raw: 1.0, At: now},
	}
	// ...but an open fail-open on gateway caps a Tier-1 service at 0.4.
	openFO := map[string]map[model.Family]bool{"checkout": {model.FamilyGateway: true}}
	sc := Build(specs, runs, openFO, now)
	if sc.Services[0].Score != FailOpenCap {
		t.Fatalf("Tier-1 service with open fail-open must cap at %v, got %v", FailOpenCap, sc.Services[0].Score)
	}
	if sc.Summary.FailOpenOpen != 1 {
		t.Fatalf("expected 1 open fail-open, got %d", sc.Summary.FailOpenOpen)
	}
	if sc.Services[0].Cells[model.FamilyGateway].State != "fail_open" {
		t.Fatalf("gateway cell should render fail_open")
	}
}

func TestTier2NotCappedByFailOpen(t *testing.T) {
	svc := model.Service{ID: "analytics", Name: "analytics", Tier: 2}
	specs := []ServiceSpec{{Service: svc, Expected: []model.Family{model.FamilyData}}}
	runs := []RunRecord{{ServiceID: "analytics", Family: model.FamilyData, Verdict: model.Pass, Raw: 1.0, At: now}}
	openFO := map[string]map[model.Family]bool{"analytics": {model.FamilyData: true}}
	sc := Build(specs, runs, openFO, now)
	// Tier-2 isn't subject to the hard 0.4 cap, but the cell still renders fail_open.
	if sc.Services[0].Cells[model.FamilyData].State != "fail_open" {
		t.Fatal("data cell should render fail_open")
	}
	if sc.Services[0].Score == FailOpenCap {
		t.Fatal("tier-2 service must not be hard-capped at 0.4 by the tier-1 rule")
	}
}
