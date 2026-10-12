// Package evidence produces self-describing, independently verifiable evidence
// receipts for every run. Given a receipt and the public key, an auditor can
// confirm what ran, who approved it, under which policy, what was observed, and
// what was concluded — without trusting this platform's UI or database.
package evidence

import (
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// SchemaVersion is the receipt schema version.
const SchemaVersion = "1.0"

// Receipt is the signed record of one security chaos run.
type Receipt struct {
	ReceiptID     string `json:"receipt_id"`
	SchemaVersion string `json:"schema_version"`

	Run        ReceiptRun    `json:"run"`
	Governance Governance    `json:"governance"`
	Preflight  Preflight     `json:"preflight"`
	Timeline   []Event       `json:"timeline"`
	Ladder     model.Ladder  `json:"ladder"`
	Verdict    model.Verdict `json:"verdict"`

	FindingID       string   `json:"finding_id,omitempty"`
	ControlFamilies []string `json:"control_families,omitempty"`
	AttackIDs       []string `json:"attack_ids,omitempty"`
	OwaspIDs        []string `json:"owasp_ids,omitempty"`

	// Seq is a monotonic sequence number within the ledger (genesis = 0). Together
	// with PrevHash and the signed ledger head it makes the ledger reorder- and
	// tail-truncation-evident (see ledger.go and build-plan.md §8 for the limits).
	Seq int64 `json:"seq"`

	// PrevHash chains this receipt to the previous one in the ledger (empty for the
	// genesis entry). Removing or reordering a receipt breaks the chain.
	PrevHash string `json:"prev_hash"`

	// Signature is populated by the Signer. It is EXCLUDED from the content hash it
	// signs (see Signer.contentHash), so it can be attached after hashing.
	Signature *Signature `json:"signature,omitempty"`
}

// ReceiptRun identifies the run and its target.
type ReceiptRun struct {
	ID                string        `json:"id"`
	UUID              string        `json:"uuid"`
	DefinitionID      string        `json:"definition_id"`
	DefinitionVersion int           `json:"definition_version"`
	ManifestHash      string        `json:"manifest_hash"`
	Env               model.Env     `json:"env"`
	Ring              model.Ring    `json:"ring,omitempty"`
	DC                string        `json:"dc,omitempty"`
	Cluster           string        `json:"cluster,omitempty"`
	Namespace         string        `json:"namespace,omitempty"`
	Service           model.Service `json:"service"`
}

// Governance records the safety decision that permitted the run.
type Governance struct {
	PolicyVersion   string           `json:"policy_version"`
	SafetyDecision  SafetyDecision   `json:"safety_decision"`
	Approvals       []model.Approval `json:"approvals,omitempty"`
	Capability      *Capability      `json:"capability,omitempty"`
	Suppressions    []Suppression    `json:"suppressions,omitempty"`
	BlackoutChecked bool             `json:"blackout_checked"`
}

// SafetyDecision is the summary of the governor's decision for the receipt.
type SafetyDecision struct {
	Allowed        bool     `json:"allowed"`
	Refusals       []string `json:"refusals"`
	GatesEvaluated int      `json:"gates_evaluated"`
}

// Capability is the per-run, scoped, time-boxed grant minted for the run.
type Capability struct {
	ID        string     `json:"id"`
	Scopes    []string   `json:"scopes"`
	IssuedAt  time.Time  `json:"issued_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// Suppression records an alert suppression registered for the run.
type Suppression struct {
	Rule       string     `json:"rule"`
	TTLSeconds int        `json:"ttl_s"`
	ExpiredAt  *time.Time `json:"expired_at,omitempty"`
}

// Preflight captures the steady-state verification performed before injection.
type Preflight struct {
	SteadyStateHeld bool         `json:"steady_state_held"`
	Probes          []ProbeCheck `json:"probes"`
}

// ProbeCheck is one steady-state probe result.
type ProbeCheck struct {
	Query    string `json:"query"`
	Result   string `json:"result"`
	Expected string `json:"expected"`
	OK       bool   `json:"ok"`
}

// Event is a timeline entry in the run's lifecycle.
type Event struct {
	TS     time.Time   `json:"ts"`
	Phase  model.Phase `json:"phase"`
	Detail string      `json:"detail,omitempty"`
}

// Signature is a detached Ed25519 signature over the receipt's content hash.
type Signature struct {
	Alg         string    `json:"alg"` // "Ed25519"
	KeyID       string    `json:"key_id"`
	ContentHash string    `json:"content_hash"` // sha256 of the canonical receipt (sans signature)
	Value       string    `json:"value"`        // base64 Ed25519 signature over ContentHash bytes
	SignedAt    time.Time `json:"signed_at"`
}
