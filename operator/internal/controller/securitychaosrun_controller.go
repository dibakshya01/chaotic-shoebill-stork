// Package controller holds the reconciler skeletons. The SecurityChaosRun reconciler
// is the ONLY door to injection on-cluster: it delegates authorization to the control
// plane's Safety Governor and never detonates on its own. This file is scaffolding —
// see operator/README.md for how to generate deepcopy, build, and run it.
package controller

import (
	"context"

	v1alpha1 "github.com/dibakshya01/chaotic-shoebill-stork/operator/api/v1alpha1"
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
	if run.Status.Phase == "Completed" || run.Status.Phase == "Refused" {
		return ctrl.Result{}, nil // terminal
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
		return ctrl.Result{}, r.Status().Update(ctx, &run)
	}

	// EXECUTE: the control plane runs the full loop and returns the signed verdict.
	status, err := r.ControlPlane.Execute(ctx, run, exp)
	if err != nil {
		return ctrl.Result{}, err
	}
	run.Status = status
	run.Status.Phase = "Completed"
	return ctrl.Result{}, r.Status().Update(ctx, &run)
}

// SetupWithManager wires the reconciler to the manager.
func (r *SecurityChaosRunReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.SecurityChaosRun{}).
		Complete(r)
}
