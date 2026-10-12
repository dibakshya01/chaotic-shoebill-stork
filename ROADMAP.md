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

## Next — the Kubernetes operator (real in-cluster detonation)

The highest-value next milestone. The CRDs are ready to apply and the controller is scaffolded.

- [ ] Implement the real infrastructure detonators with `client-go`: `k8s` (container-kill, pod
      delete), `istio` (PeerAuthentication / AuthorizationPolicy drift & bypass), `netpol`
      (NetworkPolicy isolation), `cert`.
- [ ] Wire the controller's `ControlPlane` interface to the Safety Governor + engine, and mint the
      per-experiment, time-boxed, narrowly-scoped identity (never a standing grant).
- [ ] The agent-side **autonomous dead-man's switch** (`agent.DeadMansSwitch.Run`) running
      independently of the control plane — this is where "cannot cause an incident by being down"
      becomes true on live infrastructure. (In the in-process engine today it is a synchronous
      forced-revert; the autonomous silence-detector is unit-tested but not yet driven.)
- [ ] A Helm chart + RBAC, and end-to-end tests against a `kind` cluster (controller-runtime + envtest).

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
- [ ] Scheduler + Flow/CI gate integration; a ServiceHub-style posture card.

Contributions welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).
