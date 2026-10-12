# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/), and this project aims to follow
[Semantic Versioning](https://semver.org/) from its first tagged release.

## [Unreleased]

### Added — initial MVP

- **Safety Governor** — ten ordered gates (all evaluated, never short-circuited), a native
  blast-radius policy engine with per-environment envelopes, suppression safety, and governor-minted,
  Ed25519-signed, scoped, time-boxed capabilities (the only door to injection).
- **Verification engine** — the five-stage control-response ladder, UUID correlation, a total
  adjudication truth table (`FAIL_OPEN` dominates; `INCONCLUSIVE` never becomes `PASS`), and
  pluggable signal sources (Prometheus, Alertmanager, HTTP, mock).
- **Detonators** — a capability-verifying interface with `mock`, `observe`, and `http` built-ins.
- **Control Efficacy Score** — freshness decay, untested = 0.0, Tier-1 fail-open cap.
- **Evidence** — self-describing, Ed25519-signed receipts in a hash-chained ledger with a
  separately-signed head (tamper-, reorder-, and tail-truncation-evident).
- **Dead-man's switch** — reverts an injection if the control plane goes silent or the hard deadline
  passes.
- **CLI** (`shoebill`) — `demo`, `serve`, `score`, `catalog` (list/validate/show), `evidence verify`,
  `version`.
- **Offline demo** — the whole detonate → verify → score → sign loop in-process, zero dependencies.
- **REST API + embedded dashboard** — six screens (Posture Scorecard, Fail-Open Board, Run Console,
  Safety Preview, Experiment Catalog, Game Day), localhost-only.
- **Experiment catalog** — 60 experiments across 9 families, each mapped to MITRE ATT&CK, OWASP, and
  compliance control families.
- **Kubernetes operator** — ready-to-apply CRDs (`SecurityChaosExperiment`, `SecurityChaosRun`) and a
  controller skeleton (separate module).
- Documentation, a Charter, a Security policy, and a landing site.

[Unreleased]: https://github.com/dibakshya01/chaotic-shoebill-stork/commits/main
