// Package controller holds the SecurityChaosRun reconciler: the ONLY door to injection
// on-cluster. It delegates authorization to the control plane's Safety Governor and
// never detonates on its own. It also schedules recurring runs when spec.schedule is a
// cron expression. See operator/README.md to generate deepcopy, build, and run it.
package controller

import (
	"context"
	"time"

	v1alpha1 "github.com/dibakshya01/chaotic-shoebill-stork/operator/api/v1alpha1"
	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ControlPlane is the subset of the control plane the reconciler delegates to. The
// reconciler does NOT embed the engine; it calls the control plane, which owns the
// Safety Governor, so the "one door to injection" property holds across the cluster
// boundary too.
type ControlPlane interface {
	// Plan evaluates the Safety Governor for a run without detonating.
	Plan(ctx context.Context, run v1alpha1.SecurityChaosRun, exp v1alpha1.SecurityChaosExperiment) (allowed bool, refusals []string, err error)
	// Execute runs the full loop (only reachable after an allowed Plan) and returns
	// the terminal status.
	Execute(ctx context.Context, run v1alpha1.SecurityChaosRun, exp v1alpha1.SecurityChaosExperiment) (v1alpha1.SecurityChaosRunStatus, error)
}

// SecurityChaosRunReconciler reconciles a SecurityChaosRun object.
type SecurityChaosRunReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	ControlPlane ControlPlane
}

// +kubebuilder:rbac:groups=chaos.shoebill.dev,resources=securitychaosruns,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=chaos.shoebill.dev,resources=securitychaosruns/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=chaos.shoebill.dev,resources=securitychaosexperiments,verbs=get;list;watch

// Reconcile drives a run: resolve its experiment, ask the control plane to GATE it,
// and — only if allowed — execute and record the verdict. It never injects directly.
func (r *SecurityChaosRunReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var run v1alpha1.SecurityChaosRun
	if err := r.Get(ctx, req.NamespacedName, &run); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	scheduled := run.Spec.Schedule != ""
	now := time.Now()

	// One-shot runs are terminal once done.
	if !scheduled && (run.Status.Phase == "Completed" || run.Status.Phase == "Refused") {
		return ctrl.Result{}, nil
	}

	// Scheduled runs: on first sight, compute the next fire time and wait for it; while
	// it is in the future, keep waiting rather than running.
	if scheduled {
		if run.Status.NextRunTime == nil {
			next, err := nextRun(run.Spec.Schedule, now)
			if err != nil {
				run.Status.Phase = "Refused"
				run.Status.Refusals = []string{"invalid schedule: " + err.Error()}
				return ctrl.Result{}, r.Status().Update(ctx, &run)
			}
			run.Status.Phase = "Scheduled"
			run.Status.NextRunTime = &metav1.Time{Time: next}
			if err := r.Status().Update(ctx, &run); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: time.Until(next)}, nil
		}
		if wait := run.Status.NextRunTime.Time.Sub(now); wait > 0 {
			return ctrl.Result{RequeueAfter: wait}, nil
		}
		// else: it is time to run — fall through to gate + execute.
	}

	// Resolve the referenced experiment definition.
	var exp v1alpha1.SecurityChaosExperiment
	key := client.ObjectKey{Namespace: run.Namespace, Name: run.Spec.ExperimentRef}
	if err := r.Get(ctx, key, &exp); err != nil {
		run.Status.Phase = "Refused"
		run.Status.Refusals = []string{"experiment not found: " + run.Spec.ExperimentRef}
		_ = r.Status().Update(ctx, &run)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// GATE: the control plane's Safety Governor is the only authorization to inject.
	allowed, refusals, err := r.ControlPlane.Plan(ctx, run, exp)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !allowed {
		run.Status.Phase = "Refused"
		run.Status.Refusals = refusals
		l.Info("run refused by safety governor", "run", run.Name, "refusals", len(refusals))
		return r.finish(ctx, &run, scheduled, now)
	}

	// EXECUTE: the control plane runs the full loop and returns the signed verdict.
	status, err := r.ControlPlane.Execute(ctx, run, exp)
	if err != nil {
		return ctrl.Result{}, err
	}
	run.Status = status
	run.Status.Phase = "Completed"
	return r.finish(ctx, &run, scheduled, now)
}

// finish persists the status and, for scheduled runs, records this run's time and
// schedules the next one (requeuing accordingly). The full run history lives in the
// signed evidence ledger; the status reflects the latest run.
func (r *SecurityChaosRunReconciler) finish(ctx context.Context, run *v1alpha1.SecurityChaosRun, scheduled bool, now time.Time) (ctrl.Result, error) {
	if !scheduled {
		return ctrl.Result{}, r.Status().Update(ctx, run)
	}
	run.Status.LastRunTime = &metav1.Time{Time: now}
	next, err := nextRun(run.Spec.Schedule, now)
	if err != nil {
		return ctrl.Result{}, r.Status().Update(ctx, run)
	}
	run.Status.NextRunTime = &metav1.Time{Time: next}
	if uerr := r.Status().Update(ctx, run); uerr != nil {
		return ctrl.Result{}, uerr
	}
	return ctrl.Result{RequeueAfter: time.Until(next)}, nil
}

// nextRun computes the next fire time of a standard 5-field cron expression after t.
func nextRun(schedule string, t time.Time) (time.Time, error) {
	sched, err := cron.ParseStandard(schedule)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(t), nil
}

// SetupWithManager wires the reconciler to the manager.
func (r *SecurityChaosRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SecurityChaosRun{}).
		Complete(r)
}
