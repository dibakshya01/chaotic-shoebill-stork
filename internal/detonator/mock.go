package detonator

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/sim"
)

// Mock is the in-process detonator used by the demo and tests. It injects a condition
// into a shared mock environment (which the mock signal source then observes), so the
// whole loop is causal and offline. It still enforces the capability gate in full.
type Mock struct {
	env *sim.MockEnv
	pub ed25519.PublicKey
	now func() time.Time
}

// NewMock builds a mock detonator bound to an environment and the governor's public
// key (used to verify every capability it is handed).
func NewMock(env *sim.MockEnv, governorPub ed25519.PublicKey, now func() time.Time) *Mock {
	if now == nil {
		now = time.Now
	}
	return &Mock{env: env, pub: governorPub, now: now}
}

// Kind implements Detonator.
func (m *Mock) Kind() string { return "mock" }

// Plan implements Detonator.
func (m *Mock) Plan(_ context.Context, spec Spec) (Plan, error) {
	return Plan{
		Detonator:  "mock",
		Summary:    fmt.Sprintf("inject a simulated condition on %s", spec.Target),
		Actions:    []string{"mark condition active in mock environment", "weave run uuid into artifacts"},
		Reversible: true,
	}, nil
}

// Detonate verifies the capability, then injects the condition.
func (m *Mock) Detonate(_ context.Context, spec Spec, cap model.Capability) (Artifacts, error) {
	if err := cap.Authorize(m.pub, m.now(), spec.Target, "detonator:mock"); err != nil {
		return Artifacts{}, fmt.Errorf("mock detonator refused: %w", err)
	}
	m.env.Inject(cap.RunUUID)
	return Artifacts{UUID: cap.RunUUID, Fingerprints: m.Fingerprints(cap.RunUUID)}, nil
}

// Revert verifies the capability, then clears the condition. Idempotent.
func (m *Mock) Revert(_ context.Context, spec Spec, cap model.Capability) error {
	if err := cap.Authorize(m.pub, m.now(), spec.Target, "detonator:mock"); err != nil {
		return fmt.Errorf("mock detonator refused revert: %w", err)
	}
	m.env.Revert()
	return nil
}

// VerifyReverted checks reversion independently of Revert.
func (m *Mock) VerifyReverted(_ context.Context, _ Spec) (bool, error) {
	return m.env.Reverted(), nil
}

// Fingerprints declares where the uuid appears.
func (m *Mock) Fingerprints(uuid string) []Fingerprint {
	return []Fingerprint{
		{Kind: "label", Where: RunLabel, Value: uuid},
		{Kind: "header", Where: RunHeader, Value: uuid},
	}
}
