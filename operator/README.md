# chaotic-shoebill-stork — Kubernetes operator

This directory holds the Kubernetes integration: two Custom Resource Definitions and a
controller that reconciles runs by **delegating authorization to the control plane's
Safety Governor** — the operator is one more door that opens only when the Governor
says so, never a bypass.

> **Status: scaffolding.** The CRD manifests in `config/crd/` are complete and
> ready to apply. The Go API types and the controller are provided as source for
> `controller-gen` / `kubebuilder`; they are a **separate Go module** so their heavy
> dependency tree never touches the core build, and they are **not compiled or
> end-to-end verified in this repository's CI** (that needs a cluster + envtest). The
> fully-built, fully-tested product is the control plane in the repository root.

## What's here

```
config/crd/      Ready-to-apply CustomResourceDefinitions (chaos.shoebill.dev/v1alpha1)
                   - SecurityChaosExperiment  (the reusable definition)
                   - SecurityChaosRun         (bind a definition to a target)
api/v1alpha1/    Go API types (source for controller-gen deepcopy + manifests)
internal/controller/  The SecurityChaosRun reconciler skeleton
```

## The CRDs

Apply them to any cluster:

```bash
kubectl apply -f config/crd/
```

- `SecurityChaosExperiment` (`sce`) — a declarative hypothesis + assertions. Data, not
  a payload.
- `SecurityChaosRun` (`scr`) — binds an experiment to a target. Its `.status` carries
  the phase, verdict, finding, and signed receipt id.

## Building the controller (cluster-dev environment)

1. Install [kubebuilder](https://book.kubebuilder.io/) / `controller-gen`.
2. Replace the `runtimeRawExtension` alias in `api/v1alpha1/securitychaos_types.go`
   with `*runtime.RawExtension` (it is a trimmed stand-in so the source reads cleanly
   without the full apimachinery import until you generate).
3. Generate deepcopy + manifests, then build:

```bash
cd operator
go mod tidy
controller-gen object paths=./api/...
controller-gen crd paths=./api/... output:crd:dir=config/crd
go build ./...
```

## Why delegate instead of embed

The reconciler calls a `ControlPlane` interface (`Plan` then `Execute`). The control
plane owns the Safety Governor, the signed capability minting, and the evidence ledger.
Keeping that out of the controller means the "one door to injection" invariant holds
across the cluster boundary: a reconciler cannot detonate without an allowed decision,
and the capability it would act on is signed by the Governor, not the operator.
