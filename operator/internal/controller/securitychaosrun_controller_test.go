package controller

import (
	"context"
	"testing"
	"time"

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

func TestNextRun(t *testing.T) {
	base := time.Date(2026, 1, 1, 2, 0, 0, 0, time.UTC)
	n, err := nextRun("17 3 * * *", base)
	if err != nil {
		t.Fatal(err)
	}
	if n.Hour() != 3 || n.Minute() != 17 {
		t.Fatalf("next run should be 03:17, got %v", n)
	}
	if _, err := nextRun("not a cron", base); err == nil {
		t.Fatal("an invalid cron must error")
	}
}

func TestReconcileScheduledWaitsThenRuns(t *testing.T) {
	s := newScheme(t)
	run := runCR()
	run.Spec.Schedule = "*/5 * * * *" // every 5 minutes
	cl := fake.NewClientBuilder().WithScheme(s).WithStatusSubresource(&v1.SecurityChaosRun{}).
		WithRuntimeObjects(expCR(), run).Build()
	r := &SecurityChaosRunReconciler{Client: cl, Scheme: s,
		ControlPlane: stubControlPlane{allowed: true, status: v1.SecurityChaosRunStatus{Phase: "Completed", Verdict: "PASS", RunID: "r-1"}}}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "run-1", Namespace: "ns"}}
	ctx := context.Background()

	// First reconcile: it SCHEDULES the next run and waits — it does not execute.
	res, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if res.RequeueAfter <= 0 {
		t.Fatal("a scheduled run should requeue for its next fire time")
	}
	var got v1.SecurityChaosRun
	_ = cl.Get(ctx, req.NamespacedName, &got)
	if got.Status.Phase != "Scheduled" || got.Status.NextRunTime == nil {
		t.Fatalf("expected Scheduled with a next run time, got %+v", got.Status)
	}
	if got.Status.Verdict != "" {
		t.Fatal("a scheduled run must not execute before its time")
	}

	// Force the next fire time into the past, reconcile again: now it runs + reschedules.
	past := metav1.Time{Time: time.Now().Add(-time.Minute)}
	got.Status.NextRunTime = &past
	if err := cl.Status().Update(ctx, &got); err != nil {
		t.Fatal(err)
	}
	res2, err := r.Reconcile(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	var done v1.SecurityChaosRun
	_ = cl.Get(ctx, req.NamespacedName, &done)
	if done.Status.Phase != "Completed" || done.Status.Verdict != "PASS" {
		t.Fatalf("scheduled run should have executed, got %+v", done.Status)
	}
	if done.Status.LastRunTime == nil {
		t.Fatal("a completed scheduled run should record its last run time")
	}
	if res2.RequeueAfter <= 0 {
		t.Fatal("after a scheduled run it should requeue for the next one")
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
