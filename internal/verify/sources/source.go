// Package sources provides signal sources the verification engine queries to observe
// a control's response. A source that errors or is unavailable yields INCONCLUSIVE at
// the engine level — never a PASS.
package sources

import (
	"context"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// Observation is the generic result of querying a signal source. The engine compares
// it against an assertion's Expectation.
type Observation struct {
	Found       bool    // did the query return anything
	Value       float64 // numeric value (threshold/value expectations)
	Count       int     // number of results/series (count expectations)
	MatchedUUID bool    // whether the run correlation UUID was present in the signal
	LatencyMs   int64   // latency of the signal, if known (for SLO checks)
}

// Source observes one kind of signal. uuid is the run correlation id the source
// should look for; implementations set Observation.MatchedUUID accordingly.
type Source interface {
	Name() model.SignalSource
	Query(ctx context.Context, query, uuid string) (Observation, error)
}

// Registry maps a signal-source kind to its implementation.
type Registry map[model.SignalSource]Source

// Get returns the source for a kind, or nil if unregistered.
func (r Registry) Get(s model.SignalSource) Source { return r[s] }
