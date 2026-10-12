package model

import "time"

// Env is a deployment environment. Environments form a progression: an experiment
// must PASS in each lower environment before it may run in a higher one.
type Env string

const (
	EnvLocal      Env = "local"       // in-process mock environment (develop & demo)
	EnvChaosLab   Env = "chaos-lab"   // dedicated lab namespace, purpose-built targets
	EnvPlayground Env = "playground"  // first run against real services, self-serve
	EnvStage      Env = "stage"       // primary discovery environment
	EnvPreprod    Env = "preprod"     // production-shaped, replayed traffic
	EnvProdCanary Env = "prod-canary" // final validation, single pod, dual approval
	EnvProdRing   Env = "prod-ring"   // steady-state assurance for proven experiments
)

// envRank orders environments from lowest to highest trust/impact.
var envRank = map[Env]int{
	EnvLocal: 0, EnvChaosLab: 1, EnvPlayground: 2, EnvStage: 3,
	EnvPreprod: 4, EnvProdCanary: 5, EnvProdRing: 6,
}

// Rank returns the environment's position in the progression (higher = more impactful).
func (e Env) Rank() int { return envRank[e] }

// IsProd reports whether the environment carries production impact.
func (e Env) IsProd() bool { return e == EnvProdCanary || e == EnvProdRing }

// Ring narrows a run within an environment.
type Ring string

const (
	RingCanary Ring = "canary"
	RingFull   Ring = "ring"
)

// Tier is a service criticality tier (1 = most critical).
type Tier int

// Service is the target service's identity and risk metadata (resolved from a registry).
type Service struct {
	ID             string `json:"id" yaml:"id"`
	Name           string `json:"name" yaml:"name"`
	Tier           Tier   `json:"tier" yaml:"tier"`
	Team           string `json:"team,omitempty" yaml:"team,omitempty"`
	OnPaymentsPath bool   `json:"on_payments_path,omitempty" yaml:"onPaymentsPath,omitempty"`
}

// Target binds an experiment to a concrete place to run.
type Target struct {
	Service       Service `json:"service" yaml:"service"`
	Namespace     string  `json:"namespace" yaml:"namespace"`
	Cluster       string  `json:"cluster,omitempty" yaml:"cluster,omitempty"`
	DC            string  `json:"dc,omitempty" yaml:"dc,omitempty"`
	Env           Env     `json:"env" yaml:"env"`
	Ring          Ring    `json:"ring,omitempty" yaml:"ring,omitempty"`
	FaultDomainID string  `json:"fault_domain_id,omitempty" yaml:"faultDomainId,omitempty"`
}

// Scope is the requested blast radius for a run — checked against the experiment's
// BlastRadius ceiling and the environment policy by the Safety Governor.
type Scope struct {
	PodCount      int `json:"pod_count" yaml:"podCount"`
	PctOfReplicas int `json:"pct_of_replicas" yaml:"pctOfReplicas"`
	DurationSec   int `json:"duration_seconds" yaml:"durationSeconds"`
	PctTraffic    int `json:"pct_traffic" yaml:"pctTraffic"`
}

// Approval records one sign-off. Prod requires two, from distinct teams.
type Approval struct {
	Role     string    `json:"role" yaml:"role"` // service_owner | appsec | ...
	Approver string    `json:"approver" yaml:"approver"`
	Team     string    `json:"team" yaml:"team"`
	At       time.Time `json:"at" yaml:"at"`
}

// Phase is the lifecycle phase of a run.
type Phase string

const (
	PhasePending   Phase = "Pending"
	PhaseRefused   Phase = "Refused"
	PhaseArmed     Phase = "Armed"
	PhaseInjected  Phase = "Injected"
	PhaseObserving Phase = "Observing"
	PhaseReverting Phase = "Reverting"
	PhaseVerified  Phase = "Verified"
	PhaseCompleted Phase = "Completed"
	PhaseAborted   Phase = "Aborted"
)

// Notify declares where announcements go (start, abort, result).
type Notify struct {
	Channels []string `json:"channels,omitempty" yaml:"channels,omitempty"`
}

// RunStatus is the mutable state of a run.
type RunStatus struct {
	Phase       Phase      `json:"phase" yaml:"phase"`
	RunID       string     `json:"run_id,omitempty" yaml:"runId,omitempty"`
	RunUUID     string     `json:"run_uuid,omitempty" yaml:"runUuid,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty" yaml:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty" yaml:"completedAt,omitempty"`
	Verdict     Verdict    `json:"verdict,omitempty" yaml:"verdict,omitempty"`
	Ladder      Ladder     `json:"ladder,omitempty" yaml:"ladder,omitempty"`
	FindingID   string     `json:"finding_id,omitempty" yaml:"findingId,omitempty"`
	ReceiptID   string     `json:"receipt_id,omitempty" yaml:"receiptId,omitempty"`
	Refusals    []string   `json:"refusals,omitempty" yaml:"refusals,omitempty"`
}

// Run is the intent: bind an experiment definition to a target (mirrors the
// SecurityChaosRun CRD).
type Run struct {
	Name              string         `json:"name" yaml:"name"`
	ExperimentRef     string         `json:"experiment_ref" yaml:"experimentRef"`
	ExperimentVersion int            `json:"experiment_version,omitempty" yaml:"experimentVersion,omitempty"`
	Target            Target         `json:"target" yaml:"target"`
	Scope             Scope          `json:"scope" yaml:"scope"`
	Approvals         []Approval     `json:"approvals,omitempty" yaml:"approvals,omitempty"`
	Parameters        map[string]any `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Schedule          string         `json:"schedule,omitempty" yaml:"schedule,omitempty"`
	Notify            Notify         `json:"notify,omitempty" yaml:"notify,omitempty"`
	Status            RunStatus      `json:"status" yaml:"status"`
}

// DistinctApprovalTeams returns the number of distinct teams among approvals.
func (r Run) DistinctApprovalTeams() int {
	seen := map[string]struct{}{}
	for _, a := range r.Approvals {
		if a.Team != "" {
			seen[a.Team] = struct{}{}
		}
	}
	return len(seen)
}
