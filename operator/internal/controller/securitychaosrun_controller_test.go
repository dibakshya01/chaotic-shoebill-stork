package controller

import (
	"context"
	"testing"

	v1 "github.com/dibakshya01/chaotic-shoebill-stork/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// stubControlPlane is a test double for ControlPlane so we exercise the reconcile logic
// (fetch -> gate -> execute -> status) without the real engine.
type stubControlPlane struct {
	allowed bool
	reasons []string
	status  v1.SecurityChaosRunStatus
	planErr error
	execErr error
}

func (s stubControlPlane) Plan(context.Context, v1.SecurityChaosRun, v1.SecurityChaosExperiment) (bool, []string, error) {
	return s.allowed, s.reasons, s.planErr
}
func (s stubControlPlane) Execute(context.Context, v1.SecurityChaosRun, v1.SecurityChaosExperiment) (v1.SecurityChaosRunStatus, error) {
	return s.status, s.execErr
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	if err := v1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	return s
}

func reconcileRun(t *testing.T, objs []runtime.Object, cp ControlPlane) v1.SecurityChaosRun {
	t.Helper()
	s := newScheme(t)
	b := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&v1.SecurityChaosRun{})
	for _, o := range objs {
		b = b.WithRuntimeObjects(o)
	}
	cl := b.Build()
	r := &SecurityChaosRunReconciler{Client: cl, Scheme: s, ControlPlane: cp}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "run-1", Namespace: "ns"}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var got v1.SecurityChaosRun
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "run-1", Namespace: "ns"}, &got); err != nil {
		t.Fatalf("get run: %v", err)
	}
	return got
}

func expCR() *v1.SecurityChaosExperiment {
	return &v1.SecurityChaosExperiment{
		ObjectMeta: metav1.ObjectMeta{Name: "mesh-06", Namespace: "ns"},
		Spec:       v1.SecurityChaosExperimentSpec{Family: "mesh", Description: "d"},
	}
}
func runCR() *v1.SecurityChaosRun {
	return &v1.SecurityChaosRun{
		ObjectMeta: metav1.ObjectMeta{Name: "run-1", Namespace: "ns"},
		Spec:       v1.SecurityChaosRunSpec{ExperimentRef: "mesh-06", Target: v1.Target{Env: "stage"}},
	}
}

func TestReconcileAllowedCompletes(t *testing.T) {
	got := reconcileRun(t, []runtime.Object{expCR(), runCR()},
		stubControlPlane{allowed: true, status: v1.SecurityChaosRunStatus{Phase: "Completed", Verdict: "PASS", RunID: "r-1", ReceiptID: "rc-1"}})
	if got.Status.Phase != "Completed" || got.Status.Verdict != "PASS" || got.Status.ReceiptID != "rc-1" {
		t.Fatalf("expected a completed PASS run, got %+v", got.Status)
	}
}

func TestReconcileRefusedDoesNotExecute(t *testing.T) {
	got := reconcileRun(t, []runtime.Object{expCR(), runCR()},
		stubControlPlane{allowed: false, reasons: []string{"BUDGET: duration too long"}})
	if got.Status.Phase != "Refused" {
		t.Fatalf("a refused run must not execute; phase = %q", got.Status.Phase)
	}
	if len(got.Status.Refusals) != 1 || got.Status.Refusals[0] != "BUDGET: duration too long" {
		t.Fatalf("expected the refusal recorded, got %+v", got.Status.Refusals)
	}
	if got.Status.Verdict != "" {
		t.Fatalf("a refused run has no verdict, got %q", got.Status.Verdict)
	}
}

func TestReconcileMissingExperimentRefused(t *testing.T) {
	// Only the run exists; its experiment is missing.
	got := reconcileRun(t, []runtime.Object{runCR()}, stubControlPlane{allowed: true})
	if got.Status.Phase != "Refused" {
		t.Fatalf("a run with a missing experiment must be refused, got %q", got.Status.Phase)
	}
	if len(got.Status.Refusals) == 0 {
		t.Fatal("expected a refusal reason for the missing experiment")
	}
}
