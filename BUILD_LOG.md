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
