package controlplane

import (
	"context"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/engine"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/safety"
	v1 "github.com/dibakshya01/chaotic-shoebill-stork/operator/api/v1alpha1"
)

// EngineControlPlane implements controller.ControlPlane using the core Safety Governor
// and engine. The reconciler calls Plan (gate) then, only if allowed, Execute — so the
// Governor remains the only door to injection across the cluster boundary.
type EngineControlPlane struct {
	gov safety.Governor
	eng *engine.Engine
}

// New builds an EngineControlPlane from a governor and engine.
func New(gov safety.Governor, eng *engine.Engine) *EngineControlPlane {
	return &EngineControlPlane{gov: gov, eng: eng}
}

// Plan evaluates the Safety Governor for a run without detonating.
func (c *EngineControlPlane) Plan(ctx context.Context, run v1.SecurityChaosRun, exp v1.SecurityChaosExperiment) (bool, []string, error) {
	dec, err := c.gov.Evaluate(ctx, safety.RunRequest{
		Experiment:          ToModelExperiment(exp),
		Run:                 ToModelRun(run),
		GrantedCapabilities: run.Spec.GrantedCapabilities,
		Now:                 time.Now(),
	})
	if err != nil {
		return false, nil, err
	}
	return dec.Allowed, RefusalReasons(dec), nil
}

// Execute runs the full loop (only reachable after an allowed Plan) and returns the status.
func (c *EngineControlPlane) Execute(ctx context.Context, run v1.SecurityChaosRun, exp v1.SecurityChaosExperiment) (v1.SecurityChaosRunStatus, error) {
	res, err := c.eng.Run(ctx, ToModelExperiment(exp), ToModelRun(run), run.Spec.GrantedCapabilities)
	if err != nil {
		return v1.SecurityChaosRunStatus{Phase: "Failed", Refusals: []string{err.Error()}}, err
	}
	return ToCRDStatus(res), nil
}
