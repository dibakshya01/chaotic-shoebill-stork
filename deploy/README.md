# Deploying shoebill

`shoebill` is a single static Go binary with the web dashboard embedded, so it runs anywhere — a
laptop, a VM, a container, or a Kubernetes pod. Pick the mode that fits.

## 1. Binary (any OS / VM)

```bash
go install github.com/dibakshya01/chaotic-shoebill-stork/cmd/shoebill@latest
shoebill serve --demo            # http://127.0.0.1:8099 (loopback-only by default)
```

On a cloud VM (EC2 / GCE / Azure VM), bind a reachable address only behind a trusted network or an
authenticating proxy — the API is unauthenticated:

```bash
shoebill serve --demo --addr 0.0.0.0:8099 --insecure-remote   # prints a warning; proxy it
```

## 2. Container (Docker / any cloud container runtime)

```bash
docker build -t shoebill:dev .
docker run --rm -p 8099:8099 shoebill:dev        # default CMD serves the demo dashboard
# or run the loop / validate the catalog:
docker run --rm shoebill:dev demo
docker run --rm shoebill:dev catalog validate ./catalog
```

The image is `distroless/static:nonroot` (~10 MB, no shell, non-root). A published image lands at
`ghcr.io/dibakshya01/shoebill` once you run the release workflow (see `.github/workflows/release.yml`).
Run it on ECS/Fargate, Cloud Run, Azure Container Apps, or anywhere that runs a container.

## 3. Kubernetes (GKE / EKS / AKS / kind)

The control plane deploys with the manifests here:

```bash
kubectl apply -k deploy/kubernetes/
kubectl -n shoebill port-forward svc/shoebill 8099:80     # then open http://127.0.0.1:8099
```

- Non-root, read-only root filesystem, dropped capabilities, resource limits, health probes.
- The Service is `ClusterIP` and the API is **unauthenticated** — do not expose it publicly as-is;
  front it with an authenticating ingress, or port-forward.
- Swap the `emptyDir` for a `PersistentVolumeClaim` to keep the signed evidence ledger across restarts.

### The on-cluster operator (next milestone)

Running *real* in-cluster detonations (container-kill, Istio policy, NetworkPolicy) is the job of the
Kubernetes **operator** — CRDs (`SecurityChaosExperiment`, `SecurityChaosRun`) plus a controller that
delegates authorization to the Safety Governor and acts with a narrowly-scoped, per-experiment,
time-boxed identity (never a broad standing grant). The CRDs are ready to apply
(`kubectl apply -f operator/config/crd/`); the controller is scaffolded in `operator/` and is the
focus of the next milestone (it needs a cluster + controller-runtime to build and verify). See
[`operator/README.md`](../operator/README.md) and [`ROADMAP.md`](../ROADMAP.md).

## 4. Cloud attack coverage (roadmap)

Today the **control plane** runs on any cloud (VM or managed K8s). Deep cloud *attack* coverage —
metadata-endpoint / IAM / VPC-perimeter detonators and CloudWatch / Azure Monitor / Cloud Monitoring
signal sources — is a roadmap item. The plan is to integrate **Stratus Red Team** (Go, MITRE-mapped,
AWS/Azure/GCP/K8s) behind the existing `Detonator` interface rather than reinvent it. See `ROADMAP.md`.
