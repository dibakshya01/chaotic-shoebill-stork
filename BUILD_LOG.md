# Build Log

A running, honest record of how `chaotic-shoebill-stork` was built, in order. Each entry notes what
was done, what was verified, and anything deferred.

## Phase 0 — Intake
- Clean-room, vendor-neutral build. Source material was an employer-internal design doc, used only
  as conceptual fodder; **all** internal system names and the codenames were removed. The source doc
  is git-ignored and never committed.
- Decisions: name `chaotic-shoebill-stork` (CLI `shoebill`); Apache-2.0; Go core + embedded web UI +
  offline demo + K8s CRDs/operator skeleton; push to the user's repo.
- Toolchain: Go was not installed → installed official Go 1.27.2 (checksum-verified) to `~/.local`.

## Phase 1 — Research → finding.md
- Two parallel research subagents (prior-art/competitive landscape; standards/mappings), current as
  of 2026-10-12, cited. Confirmed a clean OSS gap (controls-as-hypotheses + safety governor + signed
  evidence + efficacy scorecard exists only in closed commercial BAS or the dead-2019 ChaoSlingr).
- `finding.md` written with a competitive table, standards anchors (ATT&CK v19, OWASP 2025/2026,
  PCI/ISO/NIST/SOC2/DPDP), risks, and a synthesized direction.

## Phase 2 — Spec → build-plan.md → review
- `build-plan.md` written: thesis, goals/non-goals, personas, 21 testable FRs, §3a core definitions
  (9 families, ladder weights, 10-gate spec, Capability), architecture, 8 milestones, test strategy,
  honest limits, Definition of Done.
- **ExpertSpecReviewer cycle 1 → REVISE** (6 blocking issues). All addressed on paper:
  - B1 capability-gated detonation (not convention) · B2 full adjudication truth table + "security-
    relevant" defined · B3 ledger downscoped to tamper/reorder/truncation-evident + signed head +
    non-defenses stated · B4 9 families + ladder weights enumerated · B5 10 gates specified + dead-
    man's-switch FR/test · B6 API/UI contract fixed (endpoints, polling, localhost-only exec).
  - Non-blocking folded in: record FAIL_OPEN even when ABORTED; promotion N=14 default; demo uses an
    ephemeral key; charter-enforcement wording made precise; finding.md ATT&CK version reconciled.
- **Cycle 2 → REVISE** (all 6 original blockers resolved; one new: N1, undefined "Tier-1"). Fixed:
  defined service `tier` ∈ {1,2,3} enum, wired into FR-11 decay/cap and FR-15 validation; plus the
  expected-control-set source, key unification, and FR-9/DoD wording.
- **Cycle 3 → APPROVE.** Spec approved in 3 cycles. Two cosmetic residuals fixed (pinned the decay
  base e^(−age/τ); rule-6 "required"→"asserted"). Phase 2 complete.

## Phase 3 — Build
### M1 — Domain model + evidence  ✅ (tests green)
- `internal/model`: verdict (6, with severity ordering), ladder (5 stages + weights + raw score),
  signal/assertion/expectation, experiment (+ Duration YAML/JSON), run/target/scope/approval,
  **capability** (Ed25519-signed, run-scoped, time-boxed, `Authorize` = sig+expiry+target+scope).
- `internal/evidence`: self-describing receipt, Ed25519 signer/verifier, and a hash-chained ledger
  with monotonic seq + separately-signed head.
- Verified: `go test ./internal/model/... ./internal/evidence/...` green. Tamper flips a byte →
  verify fails; mid-chain reorder → fails; tail-truncation → fails via head mismatch; capability
  expiry/scope/forgery/tamper all rejected. `go vet` clean.

### M2–M8 ✅ (Safety Governor, verification, detonators, engine, scoring, CLI, demo, API+UI, catalog, operator)
- See commits de3b043 … a2ee52d. `go test ./... -race` green; `shoebill demo` + `evidence verify`
  reproduce the README's claims (3 fail-open, Tier-1 capped at 0.40, tamper caught); dashboard
  verified live in a browser.

## Phase 5 — Adversarial hardening
### Round 1 ✅ (fresh memory-free reviewer; findings triaged + fixed)
- **C2 (critical):** `serve --addr :PORT`/empty-host bound all interfaces. Fixed `isLoopback` so only
  an explicit loopback IP / `localhost` is loopback; `:PORT` and `0.0.0.0` now require
  `--insecure-remote`. Makefile `serve` target updated.
- **C1 (critical):** the dead-man's switch and alert suppression were implemented + unit-tested but
  never wired into `engine.Run`. Now the engine arms the dead-man's switch (and fires a forced revert
  if our own revert fails), registers validated expected-noise suppression (TTL-capped at 30m), and
  records both in the receipt. New tests assert arming, suppression recording, and the forced-revert
  path; the demo now shows suppressions.
- **M3:** http source correlation documented honestly (synchronous probe = correlated by
  construction, not coincidence).
- **M4:** http detonator given an egress guard (http/https only, no redirects, blocks
  loopback/link-local/private/unspecified at dial) with a test-only escape hatch + tests.
- **M5:** capability empty-target bypass closed; the minted `action:<x>` scope is now enforced by the
  detonators (+ tests).
- **M6:** build-plan FR-20 / DoD softened to match reality (operator is scaffolding; full reconcile
  test deferred to a cluster-dev env; CRDs validated by the `internal/crd` core test).
- **Minors:** added the `shoebill run plan` CLI (FR-17); API request body capped at 1 MiB; coverage
  raised on the critical packages (safety 84.6%, verify 84.6%, evidence 80.8%, score 93.1%);
  reconciled FR-17's CLI surface with reality.
- Verified: `go test ./... -race` green; demo + evidence verify still reproduce every claim.

### Round 2 ✅ (fresh reviewer; no Critical — R1 fixes held)
- Verdict: no exploitable hole in any shipped path; the security-critical machinery is correct and
  backed by meaningful tests. Three honesty/robustness moderates fixed:
- **CLI errors were silently swallowed** (RunE errors printed nothing) — now `cli.Execute` prints a
  clean `Error: …` line, so the serve-refusal remedy and evidence/catalog errors are visible again.
- **Dead-man's-switch claim softened** to match reality: the forced-revert-on-failed-revert is wired
  and tested; the autonomous silence-revert is tested library code, enforced on live infra by the
  operator (README + safety-model + ROADMAP aligned).
- **Egress guard made DNS-rebinding-safe**: resolve once, dial the vetted IP literal (no second
  resolution), added RFC6598 CGNAT; extended the test (IPv6 loopback, unspecified, RFC1918, CGNAT).
- Minors: `actionOf` returns "" when no action (consistent with the governor); INCONCLUSIVE no longer
  counts toward coverage; `catalog validate` detects duplicate names; added the empty-caller-target
  capability regression test; reconciled the prod-ring approvals doc with the code.
- Also (user request): containerized (Dockerfile → distroless, K8s manifests, GoReleaser + GHCR
  release workflow, ROADMAP) so it runs on any VM/cloud/cluster; added the Security Chaos Engineering
  explainer to the README + landing page.
- Verified: `go test ./... -race` green; demo/evidence/catalog/loopback-serve all reproduce.
