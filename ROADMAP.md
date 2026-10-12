# Roadmap

Honest about what's built vs. what's next. The MVP is a complete, tested control plane that runs the
full loop offline; the items below are the path to running real detonations across clusters and clouds.

## Now (MVP — shipped)

- Control plane: Safety Governor (10 gates, signed capabilities), verification engine, Control
  Efficacy Score, Ed25519 signed + hash-chained evidence ledger, findings.
- CLI (`demo`, `serve`, `run plan`, `score`, `catalog`, `evidence verify`), offline demo, REST API,
  embedded dashboard.
- 60-experiment catalog across 9 families, standards-mapped.
- Safe built-in detonators (`mock`, `observe`, `http`) and signal sources (Prometheus, Alertmanager,
  HTTP, mock).
- Runs anywhere: single static binary, container image, VM, or Kubernetes pod (see `deploy/`).

## The Kubernetes operator (in-cluster detonation)

The controller is now **implemented and unit-tested** (fake clients), not a skeleton. See
[`operator/`](operator/).

- [x] Controller `ControlPlane` wired to the Safety Governor + engine (Plan → gate, Execute → run);
      the reconciler never injects directly (fake-client reconcile tests pass).
- [x] A real, reversible `client-go` detonator: `netpol` (deny-all-ingress NetworkPolicy, labelled
      with the run UUID, revert deletes it, independently verified) — fake-clientset tests pass.
- [x] Generated CRDs + deepcopy (controller-gen), RBAC (least-privilege), manager Deployment, Dockerfile.
- [ ] **End-to-end against a real cluster** (kind/EKS/GKE/AKS) — the one thing not yet verified.
- [ ] More real detonators: `istio` (PeerAuthentication/AuthorizationPolicy drift & bypass), `dns`,
      `cert`, pod-kill.
- [ ] A fully independent agent that drives the **autonomous dead-man's switch** out-of-process —
      this is where "cannot cause an incident by being down" becomes true on live infra. (In-process
      today it is a synchronous forced-revert; the autonomous silence-detector is unit-tested.)
- [ ] A Helm chart and envtest-based e2e tests.

## Then — cloud attack coverage (AWS / Azure / GCP)

The control plane already runs on any cloud. Deep cloud *attack* coverage is additive:

- [ ] Integrate **Stratus Red Team** (Go, MITRE-mapped, AWS/Azure/GCP/K8s) behind the `Detonator`
      interface rather than reinventing cloud TTPs.
- [ ] Cloud signal sources: CloudWatch, Azure Monitor, Google Cloud Monitoring — so DETECTED/ALERTED
      assertions work against managed telemetry.
- [ ] Cloud detonators: instance-metadata access, IAM over-privilege, VPC/service-perimeter egress,
      object-storage exposure — each bounded and reversible, gated by the Governor.

## Later

- [ ] An external evidence witness / transparency log, so non-repudiation holds even against a
      compromised signing key (today's ledger is tamper/reorder/truncation-evident but single-key).
- [ ] Optional OPA/Rego policy engine behind the native blast-radius policy.
- [ ] Game Day authoring/launch from the UI (read-only in v1).
- [x] Recurring runs: a `SecurityChaosRun` with a cron `spec.schedule` is scheduled by the operator
      (wait → execute → reschedule), with full history in the signed ledger (unit-tested).
- [ ] CI-gate integration and a service-catalog posture card.

Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).
