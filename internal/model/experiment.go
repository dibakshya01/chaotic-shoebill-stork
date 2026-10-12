package model

import "time"

// Family groups experiments by the control surface they exercise.
type Family string

const (
	FamilyMesh        Family = "mesh"        // service-mesh identity & authorization
	FamilyGateway     Family = "gateway"     // gateway & edge controls
	FamilyIdentity    Family = "identity"    // identity, secrets & cryptographic lifecycle
	FamilyNetwork     Family = "network"     // network & perimeter
	FamilyData        Family = "data"        // data protection & privacy
	FamilySupplyChain Family = "supplychain" // supply chain & deployment
	FamilyDR          Family = "dr"          // DC/DR & resilience-security intersection
	FamilyDetection   Family = "detection"   // detection & response efficacy
	FamilyAI          Family = "ai"          // AI / agentic surface
)

// AllFamilies lists every family in display order.
var AllFamilies = []Family{
	FamilyMesh, FamilyGateway, FamilyIdentity, FamilyNetwork, FamilyData,
	FamilySupplyChain, FamilyDR, FamilyDetection, FamilyAI,
}

// Title returns a human-readable family name.
func (f Family) Title() string {
	switch f {
	case FamilyMesh:
		return "Service Mesh"
	case FamilyGateway:
		return "Gateway & Edge"
	case FamilyIdentity:
		return "Identity & Secrets"
	case FamilyNetwork:
		return "Network & Perimeter"
	case FamilyData:
		return "Data Protection"
	case FamilySupplyChain:
		return "Supply Chain"
	case FamilyDR:
		return "DR & Resilience"
	case FamilyDetection:
		return "Detection & Response"
	case FamilyAI:
		return "AI / Agentic"
	default:
		return string(f)
	}
}

// Mode controls whether an experiment injects a condition or only observes.
type Mode string

const (
	ModeInject      Mode = "inject"       // actively detonate a bounded condition
	ModeObserveOnly Mode = "observe_only" // read-only: drift detection, policy diff (always safe)
)

// BlastRadiusClass is a coarse label for catalog filtering and governor policy.
type BlastRadiusClass string

const (
	BlastLow    BlastRadiusClass = "low"
	BlastMedium BlastRadiusClass = "medium"
	BlastHigh   BlastRadiusClass = "high"
)

// Hypothesis states what must remain true and how to measure it right now.
type Hypothesis struct {
	Statement   string  `json:"statement" yaml:"statement"`
	SteadyState []Probe `json:"steady_state" yaml:"steadyState"`
}

// Injection declares how the adversarial condition is produced and reverted.
type Injection struct {
	Detonator string         `json:"detonator" yaml:"detonator"` // detonator kind: mock|http|exec|sim|k8s|...
	Mode      Mode           `json:"mode" yaml:"mode"`
	Spec      map[string]any `json:"spec,omitempty" yaml:"spec,omitempty"` // detonator-specific parameters
	// Duration is how long the condition is held.
	Duration Duration `json:"duration" yaml:"duration"`
	// Reversible MUST be true for any inject-mode experiment; the governor refuses otherwise.
	Reversible bool `json:"reversible" yaml:"reversible"`
	// RevertVerification checks (via a path independent of Revert) that the system returned
	// to its pre-injection state.
	RevertVerification *Probe `json:"revert_verification,omitempty" yaml:"revertVerification,omitempty"`
}

// BlastRadius is the per-experiment ceiling the governor enforces against env policy.
type BlastRadius struct {
	MaxPods                      int  `json:"max_pods" yaml:"maxPods"`
	MaxPctReplicas               int  `json:"max_pct_replicas" yaml:"maxPctReplicas"`
	MaxDurationSeconds           int  `json:"max_duration_seconds" yaml:"maxDurationSeconds"`
	MaxTrafficPct                int  `json:"max_traffic_pct" yaml:"maxTrafficPct"`
	RequiresDistinctFaultDomains bool `json:"requires_distinct_fault_domains" yaml:"requiresDistinctFaultDomains"`
}

// Promotion is the environment-progression gate: an experiment cannot run in an
// environment until it has PASSed in every lower environment within WithinDays.
type Promotion struct {
	RequiresPassIn []Env `json:"requires_pass_in,omitempty" yaml:"requiresPassIn,omitempty"`
	WithinDays     int   `json:"within_days,omitempty" yaml:"withinDays,omitempty"`
}

// ExperimentMeta carries identity and catalog labels.
type ExperimentMeta struct {
	Name    string `json:"name" yaml:"name"`
	Version int    `json:"version" yaml:"version"`
	Title   string `json:"title,omitempty" yaml:"title,omitempty"`
}

// Experiment is the reusable definition (mirrors the SecurityChaosExperiment CRD).
// It is DATA: a declarative hypothesis and its assertions, never a payload with side effects.
type Experiment struct {
	APIVersion string         `json:"api_version,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string         `json:"kind,omitempty" yaml:"kind,omitempty"`
	Meta       ExperimentMeta `json:"meta" yaml:"metadata"`

	Family           Family           `json:"family" yaml:"family"`
	BlastRadiusClass BlastRadiusClass `json:"blast_radius_class" yaml:"blastRadiusClass"`
	Description      string           `json:"description" yaml:"description"`

	// Standards mappings — every experiment is anchored to authoritative IDs.
	Controls        []string `json:"controls" yaml:"controls"`
	AttackIDs       []string `json:"attack_ids" yaml:"attackIds"`             // MITRE ATT&CK technique IDs
	OwaspIDs        []string `json:"owasp_ids" yaml:"owaspIds"`               // OWASP IDs (API/Top10/LLM/ASI/AISVS)
	ControlFamilies []string `json:"control_families" yaml:"controlFamilies"` // ISO/PCI/NIST/SOC2/DPDP IDs

	Hypothesis  Hypothesis  `json:"hypothesis" yaml:"hypothesis"`
	Injection   Injection   `json:"injection" yaml:"injection"`
	Assertions  []Assertion `json:"assertions" yaml:"assertions"`
	BlastRadius BlastRadius `json:"blast_radius" yaml:"blastRadius"`
	// ExpectedNoise is the set of alert rules suppressed during the run. It can NEVER
	// intersect the rules this experiment asserts on (schema-enforced in validation).
	ExpectedNoise []string  `json:"expected_noise,omitempty" yaml:"expectedNoise,omitempty"`
	Promotion     Promotion `json:"promotion" yaml:"promotion"`
	Capabilities  []string  `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
}

// AssertedAlertRules returns the alert rules this experiment asserts on (used by the
// suppressor to refuse suppressing an assertion).
func (e Experiment) AssertedAlertRules() []string {
	var out []string
	for _, a := range e.Assertions {
		if a.Source == SourceAlertmanager {
			out = append(out, a.Query)
		}
	}
	return out
}

// Duration is a time.Duration that marshals to/from a human string ("45s", "2m") in YAML/JSON.
type Duration time.Duration

func (d Duration) String() string { return time.Duration(d).String() }

func (d Duration) MarshalYAML() (any, error) { return time.Duration(d).String(), nil }

func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Duration(d).String() + `"`), nil
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) >= 2 && s[0] == '"' {
		s = s[1 : len(s)-1]
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}
