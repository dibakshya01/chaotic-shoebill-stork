// Package safety is the Safety Governor: the single, auditable door to injection.
// Nothing in the platform may dispatch to a detonator without a Decision{Allowed:true}
// from here, and the Capability minted on an Allowed decision is what each detonator
// verifies before acting (making the gate structural, not advisory).
//
// Design goals (do not weaken): every refusal is explained with a remedy; every
// decision is reproducible (PolicyVersion is recorded); and all ten gates are
// evaluated so the caller sees the whole envelope at once rather than fixing one
// gate at a time.
package safety

import (
	"context"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// GateID identifies one of the ten safety gates.
type GateID string

const (
	GateKillSwitch  GateID = "KILLSWITCH"
	GateSEVActive   GateID = "SEV_ACTIVE"
	GateBlackout    GateID = "BLACKOUT"
	GatePromotion   GateID = "PROMOTION"
	GateCapability  GateID = "CAPABILITY"
	GateApproval    GateID = "APPROVAL"
	GateBudget      GateID = "BUDGET"
	GateFaultDomain GateID = "FAULT_DOMAIN"
	GateConcurrency GateID = "CONCURRENCY"
	GateSteadyState GateID = "STEADY_STATE"
)

// Gates is the canonical evaluation order. All are evaluated (never short-circuited).
var Gates = []GateID{
	GateKillSwitch, GateSEVActive, GateBlackout, GatePromotion, GateCapability,
	GateApproval, GateBudget, GateFaultDomain, GateConcurrency, GateSteadyState,
}

// Refusal is one reason a run was refused, with a remedy the user can act on.
type Refusal struct {
	Gate         GateID     `json:"gate"`
	Reason       string     `json:"reason"`
	Remedy       string     `json:"remedy"`
	NextEligible *time.Time `json:"next_eligible,omitempty"`
}

// Constraints is the narrowed scope a run MUST obey if allowed (the tighter of the
// environment envelope and the experiment's declared blast radius).
type Constraints struct {
	MaxPods            int `json:"max_pods"`
	MaxPctReplicas     int `json:"max_pct_replicas"`
	MaxDurationSeconds int `json:"max_duration_seconds"`
	MaxTrafficPct      int `json:"max_traffic_pct"`
}

// ProbeResult is the outcome of one steady-state probe at preflight time.
type ProbeResult struct {
	Query    string `json:"query"`
	Expected string `json:"expected"`
	Result   string `json:"result"`
	OK       bool   `json:"ok"`
}

// SteadyStateProof records whether the system was in steady state at preflight.
type SteadyStateProof struct {
	Held   bool          `json:"held"`
	Probes []ProbeResult `json:"probes"`
}

// Decision is the governor's verdict on a run request.
type Decision struct {
	Allowed        bool              `json:"allowed"`
	Refusals       []Refusal         `json:"refusals"`
	Constraints    Constraints       `json:"constraints"`
	Capability     *model.Capability `json:"capability,omitempty"`
	PolicyVersion  string            `json:"policy_version"`
	Preflight      SteadyStateProof  `json:"preflight"`
	GatesEvaluated int               `json:"gates_evaluated"`
}

// RunRequest is the input to the governor.
type RunRequest struct {
	Experiment          model.Experiment
	Run                 model.Run
	GrantedCapabilities []string // capability grants the operator holds (e.g. "prod-injection")
	Now                 time.Time
}

// Governor is the single entry point to injection authorization.
type Governor interface {
	Evaluate(ctx context.Context, req RunRequest) (Decision, error)
}
