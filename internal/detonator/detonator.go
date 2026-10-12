// Package detonator defines the Detonator interface and the safe, built-in
// detonators. A detonator performs the bounded adversarial condition — but ONLY when
// handed a valid governor-minted Capability, which it verifies itself. This makes the
// Safety Governor the only door to injection structurally, not by convention.
package detonator

import (
	"context"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// RunHeader is the correlation header woven into HTTP artifacts (kept in sync with
// verify/sources.RunHeader).
const RunHeader = "X-Shoebill-Run"

// RunLabel is the Kubernetes label/annotation key carrying the run uuid.
const RunLabel = "shoebill.dev/run"

// Spec is the concrete injection parameters, resolved from an experiment + target.
type Spec struct {
	Target string         // canonical target identity the capability must match
	Action string         // the action name (e.g. "container-kill")
	Params map[string]any // detonator-specific parameters
}

// Plan is a dry-run rendering of what a detonation would do, without doing it.
type Plan struct {
	Detonator  string   `json:"detonator"`
	Summary    string   `json:"summary"`
	Actions    []string `json:"actions"`
	Reversible bool     `json:"reversible"`
}

// Fingerprint declares where the run uuid will be discoverable, so the verifier knows
// how to correlate an observed signal with this run.
type Fingerprint struct {
	Kind  string `json:"kind"`  // header | label | sni | dns | log | useragent
	Where string `json:"where"` // the field/location
	Value string `json:"value"` // the uuid
}

// Artifacts is what a detonation produced.
type Artifacts struct {
	UUID         string        `json:"uuid"`
	Fingerprints []Fingerprint `json:"fingerprints"`
	Notes        []string      `json:"notes,omitempty"`
}

// Detonator performs and reverts a bounded adversarial condition.
type Detonator interface {
	// Kind is the detonator's registry key (matches the capability scope "detonator:<kind>").
	Kind() string
	// Plan renders the exact actions without performing them.
	Plan(ctx context.Context, spec Spec) (Plan, error)
	// Detonate performs the injection. It MUST verify cap before acting and weave
	// cap.RunUUID into every artifact.
	Detonate(ctx context.Context, spec Spec, cap model.Capability) (Artifacts, error)
	// Revert is idempotent and safe to call repeatedly. It also verifies cap.
	Revert(ctx context.Context, spec Spec, cap model.Capability) error
	// VerifyReverted checks reversion via a path independent of Revert.
	VerifyReverted(ctx context.Context, spec Spec) (bool, error)
	// Fingerprints declares where the uuid will appear.
	Fingerprints(uuid string) []Fingerprint
}

// Registry maps a detonator kind to its implementation.
type Registry map[string]Detonator

// Get returns the detonator for a kind, or nil.
func (r Registry) Get(kind string) Detonator { return r[kind] }
