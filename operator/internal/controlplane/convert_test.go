package controlplane

import (
	"testing"
	"time"

	"github.com/dibakshya01/chaotic-shoebill-stork/internal/engine"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/evidence"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/model"
	"github.com/dibakshya01/chaotic-shoebill-stork/internal/safety"
	v1 "github.com/dibakshya01/chaotic-shoebill-stork/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToModelExperiment(t *testing.T) {
	rf := false
	e := v1.SecurityChaosExperiment{
		ObjectMeta: metav1.ObjectMeta{Name: "mesh-06"},
		Spec: v1.SecurityChaosExperimentSpec{
			Family: "mesh", BlastRadiusClass: "high", Description: "d",
			AttackIDs: []string{"T1078"}, OwaspIDs: []string{"API5:2023"},
			Hypothesis: v1.Hypothesis{Statement: "holds", SteadyState: []v1.Probe{
				{Source: "prometheus", Query: "up", Expect: v1.Expectation{Type: "present"}},
			}},
			Injection: v1.Injection{Detonator: "netpol", Mode: "inject", Duration: "45s", Reversible: true},
			Assertions: []v1.Assertion{
				{Stage: "PREVENTED", Source: "gwlog", Query: "q", Expect: v1.Expectation{Type: "absent"}, SecurityRelevant: true, RequireUUID: &rf, Justification: "all traffic"},
			},
			BlastRadius: v1.BlastRadius{MaxPods: 1, MaxDurationSeconds: 60},
		},
	}
	m := ToModelExperiment(e)
	if m.Meta.Name != "mesh-06" || m.Family != model.FamilyMesh || !m.Injection.Reversible {
		t.Fatalf("bad conversion: %+v", m.Meta)
	}
	if time.Duration(m.Injection.Duration) != 45*time.Second {
		t.Fatalf("duration not parsed: %v", m.Injection.Duration)
	}
	if len(m.Assertions) != 1 || !m.Assertions[0].SecurityRelevant || m.Assertions[0].RequiresUUID() {
		t.Fatalf("assertion not converted: %+v", m.Assertions)
	}
	if len(m.Hypothesis.SteadyState) != 1 || m.Hypothesis.SteadyState[0].Source != model.SourcePrometheus {
		t.Fatalf("steady state not converted: %+v", m.Hypothesis.SteadyState)
	}
}

func TestToModelRunAndStatus(t *testing.T) {
	r := v1.SecurityChaosRun{
		ObjectMeta: metav1.ObjectMeta{Name: "run-1"},
		Spec: v1.SecurityChaosRunSpec{
			ExperimentRef: "mesh-06",
			Target: v1.Target{Service: v1.ServiceRef{ID: "checkout", Tier: 1, OnPaymentsPath: true},
				Namespace: "checkout-stage", Env: "prod-canary", Ring: "canary"},
			Scope:     v1.Scope{PodCount: 1, DurationSeconds: 60, PctTraffic: 1},
			Approvals: []v1.Approval{{Team: "a"}, {Team: "b"}},
		},
	}
	m := ToModelRun(r)
	if m.Target.Service.ID != "checkout" || m.Target.Service.Tier != 1 || !m.Target.Service.OnPaymentsPath {
		t.Fatalf("service not converted: %+v", m.Target.Service)
	}
	if m.Target.Env != model.EnvProdCanary || m.Scope.DurationSec != 60 {
		t.Fatalf("target/scope not converted: %+v %+v", m.Target, m.Scope)
	}
	if m.DistinctApprovalTeams() != 2 {
		t.Fatalf("approvals not converted: %+v", m.Approvals)
	}

	// ToCRDStatus for an allowed run.
	st := ToCRDStatus(engine.Result{
		Decision: safety.Decision{Allowed: true, Capability: &model.Capability{RunUUID: "u-1"}},
		RunID:    "r-1", Verdict: model.Pass, Receipt: &evidence.Receipt{ReceiptID: "rc-1"},
	})
	if st.Phase != "Completed" || st.Verdict != "PASS" || st.RunUUID != "u-1" || st.ReceiptID != "rc-1" {
		t.Fatalf("allowed status conversion wrong: %+v", st)
	}
	// ...and for a refusal.
	st2 := ToCRDStatus(engine.Result{Decision: safety.Decision{Allowed: false,
		Refusals: []safety.Refusal{{Gate: safety.GateBudget, Reason: "too long"}}}})
	if st2.Phase != "Refused" || len(st2.Refusals) != 1 {
		t.Fatalf("refusal status conversion wrong: %+v", st2)
	}
}
