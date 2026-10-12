# chaotic-shoebill-stork — Kubernetes operator

The Kubernetes integration: two CRDs and a controller that reconciles runs by
**delegating authorization to the control plane's Safety Governor** — one more door that
opens only when the Governor says so, never a bypass.

> **Status: implemented and unit-tested; not yet end-to-end-verified against a live
> cluster.** The controller, the CRD↔model bridge, and a real client-go detonator are
> built and tested with fake clients (`go test ./...` green). What still needs a real
> cluster (so it is honestly *not* claimed as verified here): applying it to a kind/EKS/
> GKE/AKS cluster and running an experiment against real workloads + real Prometheus/
> Alertmanager. This is a **separate Go module** so its heavy client-go/controller-runtime
> dependency tree never touches the core build.

## What's here

```
api/v1alpha1/          CRD Go types + generated deepcopy (controller-gen)
config/crd/            Generated CustomResourceDefinitions (ready to apply)
config/rbac.yaml       ServiceAccount + least-privilege ClusterRole + binding
config/manager.yaml    The operator Deployment (non-root, probes, resource limits)
config/kustomization.yaml   apply the whole thing
internal/controller/   SecurityChaosRun reconciler (+ fake-client tests)
internal/controlplane/ CRD <-> core-model bridge; Plan (gate) then Execute (+ tests)
internal/detonators/   the real `netpol` detonator (client-go) (+ fake-clientset tests)
cmd/manager/           the manager entrypoint (wires governor + engine + detonators)
Dockerfile             builds the manager image (from the repo root)
```

## How reconciliation works (the one-door invariant across the cluster boundary)

1. The controller fetches the `SecurityChaosRun` and its referenced
   `SecurityChaosExperiment`.
2. It calls `ControlPlane.Plan` → the **Safety Governor** evaluates all 10 gates. If
   refused, the run's status becomes `Refused` with the reasons and **nothing is
   injected**.
3. Only if allowed, it calls `ControlPlane.Execute` → the core **engine** runs the loop
   (arm → detonate → observe → revert → adjudicate → sign), and the signed verdict,
   finding and receipt id land in the run's `.status`.

The controller never injects directly; the capability each detonator acts on is signed
by the Governor, not the operator.

### Recurring runs

Set `spec.schedule` on a `SecurityChaosRun` to a standard 5-field cron expression and the
operator schedules it (wait for the next fire time → gate → execute → reschedule). Each
execution appends a signed receipt to the ledger (full history there); the status shows
the latest run plus `lastRunTime` / `nextRunTime`.

## The `netpol` detonator

A real, bounded, reversible action: it applies a **deny-all-ingress NetworkPolicy** to
the target pods (isolating them, to ask "does the control fail closed when its
dependency is unreachable?"), labels it with the run UUID, and `Revert` deletes it.
`VerifyReverted` confirms via an independent `List`. It verifies the governor-minted
capability (signature, expiry, target, action scope) before acting.

## Build, generate, deploy

Requires Go 1.24+ and (for regeneration) `controller-gen`.

```bash
cd operator
go build ./...                 # builds the whole operator
go test ./...                  # fake-client reconcile + detonator + converter tests

# regenerate deepcopy + CRDs after changing api/v1alpha1 types:
controller-gen object paths=./api/...
controller-gen crd:allowDangerousTypes=true paths=./api/... output:crd:dir=config/crd

# build the image (from the repo root so the replace to the core module resolves):
docker build -f operator/Dockerfile -t ghcr.io/dibakshya01/shoebill-operator:dev .

# deploy to a cluster:
kubectl apply -k operator/config/
```

Then create a `SecurityChaosExperiment` + a `SecurityChaosRun` (see the catalog for
experiment shapes) and watch `kubectl get securitychaosruns -o wide`.

## What's next (see ../ROADMAP.md)

More real detonators (`istio`, `dns`, `cert`), a fully independent agent that drives the
autonomous dead-man's switch (silence-revert) out-of-process, a Helm chart, and
end-to-end tests against a `kind` cluster with envtest.
