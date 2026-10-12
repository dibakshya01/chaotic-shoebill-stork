// Package prober implements the Safety Governor's SteadyStateProber over the signal
// source registry, so the STEADY_STATE gate actually measures the system right now.
package prober

import (
	"context"
	"fmt"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/safety"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/verify/sources"
)

// SourceProber evaluates steady-state probes against configured signal sources.
type SourceProber struct {
	reg sources.Registry
}

// New builds a prober over a source registry.
func New(reg sources.Registry) *SourceProber { return &SourceProber{reg: reg} }

// Probe implements safety.SteadyStateProber.
func (p *SourceProber) Probe(ctx context.Context, probes []model.Probe, _ map[string]any) safety.SteadyStateProof {
	proof := safety.SteadyStateProof{Held: true}
	for _, pr := range probes {
		res := safety.ProbeResult{Query: pr.Query, Expected: string(pr.Expect.Type)}
		src := p.reg.Get(pr.Source)
		if src == nil {
			res.Result = fmt.Sprintf("source %q not configured", pr.Source)
			proof.Held = false
			proof.Probes = append(proof.Probes, res)
			continue
		}
		obs, err := src.Query(ctx, pr.Query, "")
		if err != nil {
			res.Result = "unavailable"
			proof.Held = false
			proof.Probes = append(proof.Probes, res)
			continue
		}
		ok := evalProbe(pr.Expect, obs)
		res.OK = ok
		res.Result = fmt.Sprintf("found=%v value=%g count=%d", obs.Found, obs.Value, obs.Count)
		if !ok {
			proof.Held = false
		}
		proof.Probes = append(proof.Probes, res)
	}
	return proof
}

func evalProbe(e model.Expectation, obs sources.Observation) bool {
	switch e.Type {
	case model.ExpectPresent:
		return obs.Found
	case model.ExpectAbsent:
		return !obs.Found
	case model.ExpectThreshold:
		return obs.Found && e.Op.Compare(obs.Value, e.Value)
	case model.ExpectCount:
		op := e.Op
		if op == "" {
			op = model.OpGTE
		}
		return op.Compare(float64(obs.Count), e.Value)
	case model.ExpectValue:
		op := e.Op
		if op == "" {
			op = model.OpEQ
		}
		return obs.Found && op.Compare(obs.Value, e.Value)
	default:
		return false
	}
}
