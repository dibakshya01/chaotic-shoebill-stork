package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// --- SecurityChaosExperiment: the reusable definition --------------------------

// Expectation mirrors the control-plane Expectation.
type Expectation struct {
	Type  string  `json:"type"`
	Op    string  `json:"op,omitempty"`
	Value float64 `json:"value,omitempty"`
}

// Probe is a steady-state measurement.
type Probe struct {
	Source string      `json:"source"`
	Query  string      `json:"query"`
	Expect Expectation `json:"expect"`
}

// Hypothesis states what must hold and how to measure it.
type Hypothesis struct {
	Statement   string  `json:"statement"`
	SteadyState []Probe `json:"steadyState,omitempty"`
}

// Injection declares how the condition is produced and reverted.
type Injection struct {
	Detonator string `json:"detonator"`
	// +kubebuilder:validation:Enum=inject;observe_only
	Mode               string                `json:"mode"`
	Spec               *runtime.RawExtension `json:"spec,omitempty"`
	Duration           string                `json:"duration,omitempty"`
	Reversible         bool                  `json:"reversible,omitempty"`
	RevertVerification *Probe                `json:"revertVerification,omitempty"`
}

// Assertion is one ladder rung checked after detonation.
type Assertion struct {
	// +kubebuilder:validation:Enum=PREVENTED;DETECTED;ALERTED;PAGED;TICKETED
	Stage            string      `json:"stage"`
	Source           string      `json:"source"`
	Query            string      `json:"query"`
	Expect           Expectation `json:"expect"`
	SLOms            int64       `json:"sloMs,omitempty"`
	RequireUUID      *bool       `json:"requireUuid,omitempty"`
	Justification    string      `json:"justification,omitempty"`
	SecurityRelevant bool        `json:"securityRelevant,omitempty"`
}

// BlastRadius bounds the run.
type BlastRadius struct {
	MaxPods                      int  `json:"maxPods,omitempty"`
	MaxPctReplicas               int  `json:"maxPctReplicas,omitempty"`
	MaxDurationSeconds           int  `json:"maxDurationSeconds,omitempty"`
	MaxTrafficPct                int  `json:"maxTrafficPct,omitempty"`
	RequiresDistinctFaultDomains bool `json:"requiresDistinctFaultDomains,omitempty"`
}

// Promotion is the environment-progression gate.
type Promotion struct {
	RequiresPassIn []string `json:"requiresPassIn,omitempty"`
	WithinDays     int      `json:"withinDays,omitempty"`
}

// SecurityChaosExperimentSpec is the definition.
type SecurityChaosExperimentSpec struct {
	// +kubebuilder:validation:Enum=mesh;gateway;identity;network;data;supplychain;dr;detection;ai
	Family           string      `json:"family"`
	BlastRadiusClass string      `json:"blastRadiusClass,omitempty"`
	Description      string      `json:"description"`
	Controls         []string    `json:"controls,omitempty"`
	AttackIDs        []string    `json:"attackIds,omitempty"`
	OwaspIDs         []string    `json:"owaspIds,omitempty"`
	ControlFamilies  []string    `json:"controlFamilies,omitempty"`
	Hypothesis       Hypothesis  `json:"hypothesis"`
	Injection        Injection   `json:"injection"`
	Assertions       []Assertion `json:"assertions"`
	BlastRadius      BlastRadius `json:"blastRadius,omitempty"`
	ExpectedNoise    []string    `json:"expectedNoise,omitempty"`
	Promotion        Promotion   `json:"promotion,omitempty"`
	Capabilities     []string    `json:"capabilities,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=sce;scexp
// +kubebuilder:printcolumn:name="Family",type=string,JSONPath=`.spec.family`
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.injection.mode`

// SecurityChaosExperiment is the reusable experiment definition.
type SecurityChaosExperiment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SecurityChaosExperimentSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// SecurityChaosExperimentList contains a list of SecurityChaosExperiment.
type SecurityChaosExperimentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityChaosExperiment `json:"items"`
}

// --- SecurityChaosRun: the intent (bind definition to a target) ----------------

// ServiceRef identifies the target service.
type ServiceRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Enum=1;2;3
	Tier           int    `json:"tier,omitempty"`
	Team           string `json:"team,omitempty"`
	OnPaymentsPath bool   `json:"onPaymentsPath,omitempty"`
}

// Target binds an experiment to a place to run.
type Target struct {
	Service   ServiceRef `json:"service,omitempty"`
	Namespace string     `json:"namespace,omitempty"`
	Cluster   string     `json:"cluster,omitempty"`
	DC        string     `json:"dc,omitempty"`
	// +kubebuilder:validation:Enum=local;chaos-lab;playground;stage;preprod;prod-canary;prod-ring
	Env           string `json:"env"`
	Ring          string `json:"ring,omitempty"`
	FaultDomainID string `json:"faultDomainId,omitempty"`
}

// Scope is the requested blast radius.
type Scope struct {
	PodCount        int `json:"podCount,omitempty"`
	PctOfReplicas   int `json:"pctOfReplicas,omitempty"`
	DurationSeconds int `json:"durationSeconds,omitempty"`
	PctTraffic      int `json:"pctTraffic,omitempty"`
}

// Approval is one sign-off.
type Approval struct {
	Role     string      `json:"role,omitempty"`
	Approver string      `json:"approver,omitempty"`
	Team     string      `json:"team,omitempty"`
	At       metav1.Time `json:"at,omitempty"`
}

// SecurityChaosRunSpec is the run intent.
type SecurityChaosRunSpec struct {
	ExperimentRef       string                `json:"experimentRef"`
	ExperimentVersion   int                   `json:"experimentVersion,omitempty"`
	Target              Target                `json:"target"`
	Scope               Scope                 `json:"scope,omitempty"`
	Approvals           []Approval            `json:"approvals,omitempty"`
	Parameters          *runtime.RawExtension `json:"parameters,omitempty"`
	Schedule            string                `json:"schedule,omitempty"`
	GrantedCapabilities []string              `json:"grantedCapabilities,omitempty"`
}

// SecurityChaosRunStatus is the observed run state.
type SecurityChaosRunStatus struct {
	Phase     string   `json:"phase,omitempty"`
	RunID     string   `json:"runId,omitempty"`
	RunUUID   string   `json:"runUuid,omitempty"`
	Verdict   string   `json:"verdict,omitempty"`
	FindingID string   `json:"findingId,omitempty"`
	ReceiptID string   `json:"receiptId,omitempty"`
	Refusals  []string `json:"refusals,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=scr;scrun
// +kubebuilder:printcolumn:name="Experiment",type=string,JSONPath=`.spec.experimentRef`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Verdict",type=string,JSONPath=`.status.verdict`

// SecurityChaosRun binds an experiment to a target.
type SecurityChaosRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SecurityChaosRunSpec   `json:"spec,omitempty"`
	Status            SecurityChaosRunStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SecurityChaosRunList contains a list of SecurityChaosRun.
type SecurityChaosRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityChaosRun `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecurityChaosExperiment{}, &SecurityChaosExperimentList{},
		&SecurityChaosRun{}, &SecurityChaosRunList{})
}
