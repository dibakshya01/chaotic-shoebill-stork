package sources

import (
	"context"
	"fmt"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/sim"
)

// Mock is a signal source backed by the in-process mock environment. It powers the
// offline demo and the deterministic tests.
type Mock struct {
	env  *sim.MockEnv
	kind model.SignalSource
}

// NewMock builds a mock source over the given environment, reporting as `kind` (so one
// env can stand in for prometheus/alertmanager/etc. in the demo).
func NewMock(env *sim.MockEnv, kind model.SignalSource) *Mock {
	if kind == "" {
		kind = model.SourceMock
	}
	return &Mock{env: env, kind: kind}
}

// Name implements Source.
func (m *Mock) Name() model.SignalSource { return m.kind }

// Query implements Source. A programmed "error" response surfaces as an error so the
// engine records INCONCLUSIVE.
func (m *Mock) Query(_ context.Context, query, uuid string) (Observation, error) {
	r := m.env.Lookup(query)
	if r.Err {
		return Observation{}, fmt.Errorf("mock source %q unavailable for query %q", m.kind, query)
	}
	return Observation{
		Found:       r.Found,
		Value:       r.Value,
		Count:       r.Count,
		LatencyMs:   r.LatencyMs,
		MatchedUUID: m.env.Active() && r.Found && m.env.UUID() == uuid,
	}, nil
}
