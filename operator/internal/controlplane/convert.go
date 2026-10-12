// Package controlplane bridges the Kubernetes CRDs to the core control plane: it
// converts a SecurityChaosExperiment/Run into the core model, runs them through the
// Safety Governor and engine, and converts the result back into the CRD status. This
// is what makes the operator delegate authorization to the Governor rather than
// injecting on its own.
package controlplane

import (
	"encoding/json"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/engine"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/safety"
	v1 "github.com/dibakshya01/chaotic-shoebill-stork/operator/api/v1alpha1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// ToModelExperiment converts a CRD experiment into the core model.
func ToModelExperiment(e v1.SecurityChaosExperiment) model.Experiment {
	s := e.Spec
	return model.Experiment{
		Meta:             model.ExperimentMeta{Name: e.Name, Version: 1, Title: e.Name},
		Family:           model.Family(s.Family),
		BlastRadiusClass: model.BlastRadiusClass(s.BlastRadiusClass),
		Description:      s.Description,
		Controls:         s.Controls,
		AttackIDs:        s.AttackIDs,
		OwaspIDs:         s.OwaspIDs,
		ControlFamilies:  s.ControlFamilies,
		Hypothesis: model.Hypothesis{
			Statement:   s.Hypothesis.Statement,
			SteadyState: toProbes(s.Hypothesis.SteadyState),
		},
		Injection: model.Injection{
			Detonator:          s.Injection.Detonator,
			Mode:               model.Mode(s.Injection.Mode),
			Spec:               rawToMap(s.Injection.Spec),
			Duration:           parseDuration(s.Injection.Duration),
			Reversible:         s.Injection.Reversible,
			RevertVerification: toProbePtr(s.Injection.RevertVerification),
		},
		Assertions: toAssertions(s.Assertions),
		BlastRadius: model.BlastRadius{
			MaxPods:                      s.BlastRadius.MaxPods,
			MaxPctReplicas:               s.BlastRadius.MaxPctReplicas,
			MaxDurationSeconds:           s.BlastRadius.MaxDurationSeconds,
			MaxTrafficPct:                s.BlastRadius.MaxTrafficPct,
			RequiresDistinctFaultDomains: s.BlastRadius.RequiresDistinctFaultDomains,
		},
		ExpectedNoise: s.ExpectedNoise,
		Promotion:     model.Promotion{RequiresPassIn: toEnvs(s.Promotion.RequiresPassIn), WithinDays: s.Promotion.WithinDays},
		Capabilities:  s.Capabilities,
	}
}

// ToModelRun converts a CRD run into the core model.
func ToModelRun(r v1.SecurityChaosRun) model.Run {
	t := r.Spec.Target
	return model.Run{
		Name:              r.Name,
		ExperimentRef:     r.Spec.ExperimentRef,
		ExperimentVersion: r.Spec.ExperimentVersion,
		Target: model.Target{
			Service: model.Service{
				ID: t.Service.ID, Name: t.Service.Name, Tier: model.Tier(t.Service.Tier),
				Team: t.Service.Team, OnPaymentsPath: t.Service.OnPaymentsPath,
			},
			Namespace: t.Namespace, Cluster: t.Cluster, DC: t.DC,
			Env: model.Env(t.Env), Ring: model.Ring(t.Ring), FaultDomainID: t.FaultDomainID,
		},
		Scope: model.Scope{
			PodCount: r.Spec.Scope.PodCount, PctOfReplicas: r.Spec.Scope.PctOfReplicas,
			DurationSec: r.Spec.Scope.DurationSeconds, PctTraffic: r.Spec.Scope.PctTraffic,
		},
		Approvals:  toApprovals(r.Spec.Approvals),
		Parameters: rawToMap(r.Spec.Parameters),
		Schedule:   r.Spec.Schedule,
	}
}

// ToCRDStatus converts an engine result (or a refusal decision) into the CRD status.
func ToCRDStatus(res engine.Result) v1.SecurityChaosRunStatus {
	st := v1.SecurityChaosRunStatus{Verdict: string(res.Verdict)}
	if !res.Decision.Allowed {
		st.Phase = "Refused"
		for _, rf := range res.Decision.Refusals {
			st.Refusals = append(st.Refusals, string(rf.Gate)+": "+rf.Reason)
		}
		return st
	}
	st.Phase = "Completed"
	st.RunID = res.RunID
	if res.Decision.Capability != nil {
		st.RunUUID = res.Decision.Capability.RunUUID
	}
	if res.Receipt != nil {
		st.ReceiptID = res.Receipt.ReceiptID
	}
	if res.Finding != nil {
		st.FindingID = res.Finding.ID
	}
	return st
}

// RefusalReasons renders a decision's refusals as human strings (for the Plan path).
func RefusalReasons(dec safety.Decision) []string {
	out := make([]string, 0, len(dec.Refusals))
	for _, r := range dec.Refusals {
		out = append(out, string(r.Gate)+": "+r.Reason)
	}
	return out
}

// --- helpers ---

func toProbes(ps []v1.Probe) []model.Probe {
	out := make([]model.Probe, 0, len(ps))
	for _, p := range ps {
		out = append(out, model.Probe{Source: model.SignalSource(p.Source), Query: p.Query, Expect: toExpect(p.Expect)})
	}
	return out
}

func toProbePtr(p *v1.Probe) *model.Probe {
	if p == nil {
		return nil
	}
	mp := model.Probe{Source: model.SignalSource(p.Source), Query: p.Query, Expect: toExpect(p.Expect)}
	return &mp
}

func toExpect(e v1.Expectation) model.Expectation {
	return model.Expectation{Type: model.ExpectationType(e.Type), Op: model.Op(e.Op), Value: e.Value}
}

func toAssertions(as []v1.Assertion) []model.Assertion {
	out := make([]model.Assertion, 0, len(as))
	for _, a := range as {
		out = append(out, model.Assertion{
			Stage: model.LadderStage(a.Stage), Source: model.SignalSource(a.Source), Query: a.Query,
			Expect: toExpect(a.Expect), SLOms: a.SLOms, RequireUUID: a.RequireUUID,
			Justification: a.Justification, SecurityRelevant: a.SecurityRelevant,
		})
	}
	return out
}

func toApprovals(as []v1.Approval) []model.Approval {
	out := make([]model.Approval, 0, len(as))
	for _, a := range as {
		out = append(out, model.Approval{Role: a.Role, Approver: a.Approver, Team: a.Team, At: a.At.Time})
	}
	return out
}

func toEnvs(es []string) []model.Env {
	out := make([]model.Env, 0, len(es))
	for _, e := range es {
		out = append(out, model.Env(e))
	}
	return out
}

func parseDuration(s string) model.Duration {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return model.Duration(d)
}

func rawToMap(r *runtime.RawExtension) map[string]any {
	if r == nil || len(r.Raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(r.Raw, &m); err != nil {
		return nil
	}
	return m
}
