package detonator

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// Observe is the detonator for observe_only experiments (drift detection, policy diff,
// coverage scans). It performs no injection at all — it is always safe to run — but
// still verifies the capability for a uniform code path. Revert is a no-op.
type Observe struct {
	pub ed25519.PublicKey
	now func() time.Time
}

// NewObserve builds an observe-only detonator.
func NewObserve(governorPub ed25519.PublicKey, now func() time.Time) *Observe {
	if now == nil {
		now = time.Now
	}
	return &Observe{pub: governorPub, now: now}
}

// Kind implements Detonator.
func (o *Observe) Kind() string { return "observe" }

// Plan implements Detonator.
func (o *Observe) Plan(_ context.Context, spec Spec) (Plan, error) {
	return Plan{
		Detonator:  "observe",
		Summary:    fmt.Sprintf("read-only observation of %s (no injection)", spec.Target),
		Actions:    []string{"query signal sources only"},
		Reversible: true,
	}, nil
}

// Detonate performs no injection; it only confirms the capability and returns artifacts.
func (o *Observe) Detonate(_ context.Context, spec Spec, cap model.Capability) (Artifacts, error) {
	if err := cap.Authorize(o.pub, o.now(), spec.Target, "detonator:observe"); err != nil {
		return Artifacts{}, fmt.Errorf("observe detonator refused: %w", err)
	}
	return Artifacts{UUID: cap.RunUUID, Fingerprints: o.Fingerprints(cap.RunUUID), Notes: []string{"observe_only: no condition injected"}}, nil
}

// Revert is a no-op (nothing was changed).
func (o *Observe) Revert(_ context.Context, _ Spec, _ model.Capability) error { return nil }

// VerifyReverted always reports reverted (nothing to undo).
func (o *Observe) VerifyReverted(_ context.Context, _ Spec) (bool, error) { return true, nil }

// Fingerprints implements Detonator.
func (o *Observe) Fingerprints(uuid string) []Fingerprint {
	return []Fingerprint{{Kind: "label", Where: RunLabel, Value: uuid}}
}
