// Package findings models the lifecycle of a finding produced by a run. A fail-open or
// a decayed detection becomes a tracked finding with an owner, not a line in a log.
package findings

import (
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
)

// Status is a finding's lifecycle state.
type Status string

const (
	Open     Status = "open"
	Accepted Status = "accepted" // risk accepted by an owner
	Resolved Status = "resolved"
)

// Severity labels a finding for triage.
type Severity string

const (
	Critical Severity = "critical"
	High     Severity = "high"
	Medium   Severity = "medium"
	Low      Severity = "low"
)

// Finding is an actionable result of a run.
type Finding struct {
	ID              string        `json:"id"`
	Verdict         model.Verdict `json:"verdict"`
	Severity        Severity      `json:"severity"`
	ServiceID       string        `json:"service_id"`
	ServiceName     string        `json:"service_name"`
	Tier            model.Tier    `json:"tier"`
	Family          model.Family  `json:"family"`
	Experiment      string        `json:"experiment"`
	Title           string        `json:"title"`
	Detail          string        `json:"detail"`
	AttackIDs       []string      `json:"attack_ids,omitempty"`
	OwaspIDs        []string      `json:"owasp_ids,omitempty"`
	ControlFamilies []string      `json:"control_families,omitempty"`
	Owner           string        `json:"owner,omitempty"`
	Team            string        `json:"team,omitempty"`
	OpenedAt        time.Time     `json:"opened_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	SeenCount       int           `json:"seen_count"`
	Status          Status        `json:"status"`
	ReceiptID       string        `json:"receipt_id,omitempty"`
}

// SeverityFor maps a verdict + tier to a triage severity.
func SeverityFor(v model.Verdict, tier model.Tier) Severity {
	switch v {
	case model.FailOpen:
		if tier == 1 {
			return Critical
		}
		return High
	case model.FailUndetected:
		if tier == 1 {
			return High
		}
		return Medium
	case model.FailSlow:
		return Medium
	default:
		return Low
	}
}

// IsOpenFailOpen reports whether this is an open fail-open finding (used by the scorer's
// Tier-1 cap).
func (f Finding) IsOpenFailOpen() bool {
	return f.Status == Open && f.Verdict == model.FailOpen
}
