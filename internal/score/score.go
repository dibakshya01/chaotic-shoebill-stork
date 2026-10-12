// Package score computes the Control Efficacy Score — a single honest number per
// service (and overall) that decays with staleness, treats an expected-but-untested
// control as 0.0 (not "unknown"), and caps a Tier-1 service at 0.4 while it has an
// open fail-open finding. Green means *currently* proven.
package score

import (
	"math"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// Freshness decay time constants (days). A Tier-1 control must be re-proven more often.
const (
	TauTier1 = 14.0
	TauOther = 30.0
	// FailOpenCap bounds a Tier-1 service score while a fail-open finding is open.
	FailOpenCap = 0.4
)

// Freshness returns e^(-ageDays/τ) in [0,1], τ keyed off the service tier.
func Freshness(ageDays float64, tier model.Tier) float64 {
	if ageDays < 0 {
		ageDays = 0
	}
	tau := TauOther
	if tier == 1 {
		tau = TauTier1
	}
	return math.Exp(-ageDays / tau)
}

// ServiceSpec is a service and the control families it is EXPECTED to pass. The
// expected set is the scorecard denominator, so an expected-but-untested family scores 0.
type ServiceSpec struct {
	Service  model.Service
	Expected []model.Family
}

// RunRecord is the latest adjudicated result for one (service, family, experiment).
type RunRecord struct {
	ServiceID  string
	Family     model.Family
	Experiment string
	Verdict    model.Verdict
	Raw        float64 // ladder.RawScore()
	At         time.Time
}

// Cell is the scorecard matrix cell for a service × family.
type Cell struct {
	Family       model.Family  `json:"family"`
	Tested       bool          `json:"tested"`
	Verdict      model.Verdict `json:"verdict,omitempty"` // worst verdict among the family's experiments
	Raw          float64       `json:"raw"`
	Score        float64       `json:"score"`
	LastRun      *time.Time    `json:"last_run,omitempty"`
	OpenFailOpen bool          `json:"open_fail_open"`
	State        string        `json:"state"` // pass|slow|untested|fail|fail_open
}

// ServiceScorecard is one row of the posture matrix.
type ServiceScorecard struct {
	Service model.Service          `json:"service"`
	Cells   map[model.Family]*Cell `json:"cells"`
	Score   float64                `json:"score"`
}

// Summary is the top-of-dashboard rollup.
type Summary struct {
	Efficacy       float64 `json:"efficacy"`        // mean service score [0,1]
	FailOpenOpen   int     `json:"fail_open_open"`  // open fail-open findings
	CoverageTested int     `json:"coverage_tested"` // expected cells that have been tested
	CoverageTotal  int     `json:"coverage_total"`  // expected cells
	Freshness      float64 `json:"freshness"`       // fraction of tested cells within their τ window
}

// Scorecard is the whole posture view.
type Scorecard struct {
	Summary  Summary             `json:"summary"`
	Services []*ServiceScorecard `json:"services"`
}

// Build computes the scorecard from the expected services, the latest run records, and
// the set of open fail-open findings keyed by (serviceID, family).
func Build(services []ServiceSpec, runs []RunRecord, openFailOpen map[string]map[model.Family]bool, now time.Time) Scorecard {
	// Group runs by service -> family -> records.
	byService := map[string]map[model.Family][]RunRecord{}
	for _, r := range runs {
		if byService[r.ServiceID] == nil {
			byService[r.ServiceID] = map[model.Family][]RunRecord{}
		}
		byService[r.ServiceID][r.Family] = append(byService[r.ServiceID][r.Family], r)
	}

	var cards []*ServiceScorecard
	var sumScores float64
	coverageTested, coverageTotal := 0, 0
	var freshTested, totalTested int
	failOpenCount := 0

	for _, spec := range services {
		card := &ServiceScorecard{Service: spec.Service, Cells: map[model.Family]*Cell{}}
		var cellScores []float64
		for _, fam := range spec.Expected {
			coverageTotal++
			cell := &Cell{Family: fam}
			records := byService[spec.Service.ID][fam]
			openFO := openFailOpen[spec.Service.ID][fam]
			cell.OpenFailOpen = openFO
			if openFO {
				failOpenCount++
			}
			if len(records) == 0 {
				cell.Tested = false
				cell.State = "untested"
				cell.Score = 0
			} else {
				coverageTested++
				totalTested++
				cell.Tested = true
				// worst verdict + mean raw + latest timestamp
				var rawSum float64
				worst := model.Pass
				var latest time.Time
				for _, r := range records {
					rawSum += r.Raw
					if r.Verdict.WorseThan(worst) {
						worst = r.Verdict
					}
					if r.At.After(latest) {
						latest = r.At
					}
				}
				cell.Verdict = worst
				cell.Raw = rawSum / float64(len(records))
				lr := latest
				cell.LastRun = &lr
				ageDays := now.Sub(latest).Hours() / 24
				cell.Score = cell.Raw * Freshness(ageDays, spec.Service.Tier)
				cell.State = stateFor(worst, openFO)
				if withinTau(ageDays, spec.Service.Tier) {
					freshTested++
				}
			}
			card.Cells[fam] = cell
			cellScores = append(cellScores, cell.Score)
		}
		card.Score = mean(cellScores)
		// Hard override: an open Tier-1 fail-open caps the service score.
		if spec.Service.Tier == 1 && anyOpenFailOpen(card) && card.Score > FailOpenCap {
			card.Score = FailOpenCap
		}
		sumScores += card.Score
		cards = append(cards, card)
	}

	fresh := 0.0
	if totalTested > 0 {
		fresh = float64(freshTested) / float64(totalTested)
	}
	eff := 0.0
	if len(cards) > 0 {
		eff = sumScores / float64(len(cards))
	}
	return Scorecard{
		Summary: Summary{
			Efficacy: eff, FailOpenOpen: failOpenCount,
			CoverageTested: coverageTested, CoverageTotal: coverageTotal,
			Freshness: fresh,
		},
		Services: cards,
	}
}

func stateFor(v model.Verdict, openFailOpen bool) string {
	if openFailOpen || v == model.FailOpen {
		return "fail_open"
	}
	switch v {
	case model.Pass:
		return "pass"
	case model.FailSlow:
		return "slow"
	case model.Inconclusive:
		return "untested" // could not observe → treat as not proven in the matrix
	default:
		return "fail"
	}
}

func withinTau(ageDays float64, tier model.Tier) bool {
	tau := TauOther
	if tier == 1 {
		tau = TauTier1
	}
	return ageDays <= tau
}

func anyOpenFailOpen(card *ServiceScorecard) bool {
	for _, c := range card.Cells {
		if c.OpenFailOpen || c.Verdict == model.FailOpen {
			return true
		}
	}
	return false
}

func mean(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}
