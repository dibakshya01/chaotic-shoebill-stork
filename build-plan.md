# Build Plan — chaotic-shoebill-stork

> Spec-driven. Nothing is built that isn't written here. When reality forces a change, this file is
> edited first, then the code. Derived from `finding.md`.

## 0. Thesis & principles

**Thesis:** Treat every security control as a testable hypothesis — safely detonate a bounded,
reversible, pre-declared adversarial condition; assert whether the control *prevented, detected,
alerted, and paged* within its SLO; sign the evidence; roll it into a per-service control-efficacy
scorecard that decays with staleness.

**Principles (decide every trade-off):**
1. **One door to injection** — the Safety Governor is the only path; nothing detonates without an
   `Allowed` decision; the platform cannot cause an incident by being down.
2. **Never lie** — `FAIL_OPEN` is its own verdict; `INCONCLUSIVE` never becomes `PASS`; *untested*
   scores `0.0`.
3. **Causality, not coincidence** — a run-UUID woven into every artifact and required by assertions.
4. **Evidence an auditor can trust without trusting us** — Ed25519-signed, self-describing receipts.
5. **Offline, deterministic, free, auditable** — the whole loop runs on a laptop, zero external
   deps; same inputs → same verdict and byte-stable receipt (modulo injected clock/uuid).

## 1. Goals & non-goals

**Goals**
- A Go control plane implementing the 7-step core loop (hypothesize → gate → arm → detonate →
  observe → revert → adjudicate) with signed evidence and efficacy scoring.
- A **Safety Governor** with 10 ordered gates, all evaluated, policy-as-data.
- A **verification engine** with the 5-stage ladder, honest adjudication, and UUID correlation.
- A pluggable **Detonator** interface with safe built-in detonators and an offline mock.
- Pluggable **signal sources** (Prometheus, Alertmanager, HTTP, mock).
- An **experiment catalog** as data (YAML), 9 families, each experiment mapped to standards.
- A **CLI** (`shoebill`) to validate, dry-run, run, score, serve, verify evidence, and demo.
- A **REST API** and an **embedded web dashboard** (6 screens) that renders real data.
- An **offline demo** that runs the whole loop in-process and populates the dashboard.
- **Kubernetes CRDs** + an operator/controller skeleton (separate module).
- Enterprise hygiene: Apache-2.0, CHARTER, AGENTS, SECURITY, CODEOWNERS, CI, tests, docs.

**Non-goals (deliberate)**
- Not a vulnerability scanner (no SAST/DAST/SCA) — it tests *runtime control behavior*.
- Not a red-team tool — no exploits, no 0-days, no lateral movement, no persistence (see CHARTER).
- Not an observability or SIEM stack — it *queries* Prometheus/Alertmanager/etc., doesn't replace.
- Not a data-exfiltration tool — data experiments use canary tokens only, never real PII.
- No prod injection by default — requires explicit, signed, time-boxed capability + dual approval.

## 2. Users & jobs-to-be-done

Primary user in one line: **an AppSec/product-security engineer who needs continuous proof that the
controls they signed off on still work — without causing an incident to find out.**

| Persona | Job hired for |
|---------|---------------|
| AppSec engineer (primary) | "Prove the controls I approved last quarter still work, with evidence and owners." |
| Detection engineer | "Prove my detection rules still fire, and measure how fast." |
| SRE / platform engineer | "Run a security game day as a pre-event gate, safely, without paging the on-call for a chaos run." |
| GRC analyst | "Get auditor-consumable, signed evidence as a by-product, not a project." |
| Security leadership | "One honest scorecard: what's proven, what's untested, what's failing open." |

## 3. Functional spec (testable requirements)

**Core loop & engine**
- **FR-1** The engine runs the 7 steps in order and short-circuits to `ABORTED` on kill switch or
  guardrail breach. *Proof:* engine test drives a full run against the mock environment.
- **FR-2** Injection is *structurally* gated, not by convention: `Detonator.Detonate` takes a
  **Capability** (not a bare UUID). A Capability is minted *only* by the Safety Governor on an
  `Allowed` decision, is signed by the governor's key, and is run-scoped and time-boxed. Every
  detonator calls `capability.Authorize(spec)` first and refuses if the capability is absent,
  expired, mis-scoped, or not signed by the trusted governor key. *Proof:* a contract test runs
  every registered detonator with (a) no capability, (b) an expired capability, (c) a capability
  scoped to a different target — each must refuse without side effects; and a test that a forged
  capability (wrong signer) is rejected.
- **FR-3** Every detonator weaves the run UUID (carried inside the Capability) into every artifact;
  a contract test enforces it for every registered detonator via `Fingerprints(uuid)`. *Proof:*
  `detonator` contract test.

**Safety Governor**
- **FR-4** `Evaluate` returns *all* refusals (never short-circuits) across the 10 gates: KILLSWITCH,
  SEV_ACTIVE, BLACKOUT, PROMOTION, CAPABILITY, APPROVAL, BUDGET, FAULT_DOMAIN, CONCURRENCY,
  STEADY_STATE. *Proof:* a case failing 3 gates returns 3 refusals, each with reason + remedy.
- **FR-5** Blast-radius budget is evaluated from policy-as-data (native engine; OPA/Rego optional),
  enforcing env-tiered envelopes (prod-canary ≤1 pod / ≤60s / ≤1% traffic / dual distinct-team
  approval; stage generous; playground near-unrestricted but time-bounded). *Proof:* policy tests.
- **FR-6** Blackout windows hard-refuse during freeze periods; promotion gate refuses an experiment
  that has not PASSed in every lower env within N days. *Proof:* governor tests.
- **FR-7** Capabilities are per-run, scoped, time-boxed; the platform cannot mint a capability the
  decision did not authorize. *Proof:* capability test.
- **FR-8** Suppression requests are rejected if they intersect the experiment's asserted alert
  rules, and TTL is capped at 30m. *Proof:* suppressor test.

**Verification & scoring**
- **FR-9** Adjudication is a total function from per-stage results to one verdict, implemented
  exactly as the truth table in §3a. "Security-relevant" is an explicit boolean on the assertion
  (`securityRelevant: true`), set for any assertion whose `absent` expectation, if matched, means a
  control permitted something it should have denied. The resolution order is evaluated top-down and
  the first matching rule wins:
  1. run was aborted → **ABORTED** (but any fail-open observed *before* the abort still opens a
     FAIL_OPEN finding — see FR-11a).
  2. any assertion with `expect: absent` + `securityRelevant: true` **matched** (the forbidden thing
     happened) → **FAIL_OPEN**.
  3. any **asserted** stage's signal source **errored / was unavailable** → **INCONCLUSIVE**
     (never downgraded to PASS). ("Asserted" = the experiment declared an assertion for that stage;
     there is no separate "required" notion.)
  4. every asserted stage **matched within its SLO** → **PASS**.
  5. every asserted stage matched but **≥1 exceeded SLO** → **FAIL_SLOW**.
  6. otherwise (≥1 asserted stage did not match at all) → **FAIL_UNDETECTED**.
  A stage that was *not asserted* is absent from the ladder and ignored. A stage *asserted but
  unobserved because its source errored* is `INCONCLUSIVE` (rule 3). A stage *skipped because an
  earlier asserted stage failed* is `SKIP` and counts as "did not match" for rule 6.
  *Proof:* a table-driven test enumerating every rule, incl. FAIL_OPEN dominance over a stage that
  otherwise PASSed, and INCONCLUSIVE never becoming PASS.
- **FR-10** Assertions correlate on the run UUID by default; `requireUuid:false` requires a written
  `justification` (schema-enforced). *Proof:* catalog validation test.
- **FR-11** Control Efficacy Score = raw(weighted ladder, weights in §3a) × freshness, where
  `freshness = e^(−days_since_last_run / τ)`, τ=14 for Tier-1 and 30 for Tier-2/3; untested expected
  control = 0.0; an open Tier-1 FAIL_OPEN caps the service score at 0.4. *Proof:* scoring tests
  covering decay, cap, and untested=0.
- **FR-11a** A fail-open *observation* is never swallowed by a later abort: if any stage recorded a
  matched security-relevant `absent` assertion, a FAIL_OPEN finding is opened even when the run
  verdict is ABORTED. *Proof:* test aborts a run after a fail-open observation and asserts the
  finding still opens.

**Evidence** (claims deliberately scoped — see §8 for what this does NOT defend against)
- **FR-12** Each run emits a self-describing receipt (run, governance, preflight, timeline, ladder,
  verdict, finding, standards mappings) carrying a monotonic `seq`, a `prev_hash` chaining it to the
  prior receipt, and an Ed25519 signature over the canonical content hash. *Proof:* sign test.
- **FR-13** `shoebill evidence verify` validates every signature against a trusted public key and
  detects any content tampering (flipped verdict, altered ladder, edited approval). *Proof:* tamper
  test flips a byte → verify fails with `ErrBadSignature`.
- **FR-14** The ledger is a **tamper-evident, reorder-evident hash chain**: each receipt chains the
  previous receipt's content hash and carries a monotonic sequence number, and the verifier checks
  chain linkage, sequence monotonicity, and every signature. To also catch **tail truncation**
  (dropping the last N receipts — which a bare chain cannot detect), the ledger maintains a
  separately-stored, signed **head** record (highest seq + its hash); `verify` fails if the ledger's
  last entry does not match the signed head. *Proof:* (a) a reorder/removal-in-the-middle test fails
  verification via chain break; (b) a tail-truncation test (drop the last entry) fails verification
  via head mismatch. *Explicit non-defense (documented, not a bug):* a holder of the signing key can
  rewrite and re-sign the whole chain + head; defending against that needs an external witness /
  transparency log, which is out of scope for v1 and noted in §8.

**Catalog**
- **FR-15** The catalog loads from YAML into the model and validates: schema; standards IDs present
  (≥1 ATT&CK + ≥1 OWASP/compliance); inject-mode experiments declare `reversible: true`;
  suppression (`expectedNoise`) ∩ asserted alert rules = ∅; blast-radius within env bounds; every
  `requireUuid:false` assertion has a non-empty `justification`. The registry (`services.yaml`) is
  validated separately: every service carries a `tier` in {1,2,3}. *Proof:* `shoebill catalog
  validate ./catalog` passes for all shipped experiments; malformed fixtures (missing tier, bad
  suppression, missing justification) each fail with a precise error.
- **FR-16** ≥ 40 experiments across all 9 families ship, each with ATT&CK + OWASP + compliance IDs.

**CLI** (`shoebill <command>`)
- **FR-17** `version`, `catalog list|validate|show`, `run plan` (dry-run through the governor, prints
  the decision + all 10 gates), `score`, `evidence verify`, `serve [--demo] [--addr]`, `demo`.
  Full run *execution* is exercised in-process by `demo` and on-cluster by the operator — there is no
  standalone `run exec` in v1 (execution is deliberately not a one-liner CLI against arbitrary live
  targets). *Proof:* CLI runs + `make demo`.

**API + UI** — the API binds to **localhost by default and is unauthenticated**; it is an operator
console, not a multi-tenant control surface. Experiment *execution* is CLI-first; the API exposes it
only for the local demo and refuses non-loopback binds without an explicit `--insecure-remote` flag
(documented as unsupported). This is a deliberate safety choice, stated plainly.
- **FR-18** REST API (JSON): reads — `GET /api/v1/scores`, `/experiments`, `/experiments/{id}`,
  `/runs`, `/runs/{id}`, `/findings`, `/findings/failopen`, `/gamedays`, `/gamedays/{id}`,
  `/healthz`, `/version`. Writes — `POST /api/v1/experiments:validate` (validate a composed
  experiment, returns errors), `POST /api/v1/runs:plan` (safety preview: runs the governor, returns
  the full 10-gate decision without detonating), `POST /api/v1/runs` (create + execute a run;
  **loopback-only**, used by the demo). The Live Run Console **polls** `GET /api/v1/runs/{id}` (which
  returns the evolving phase + ladder); no websocket/SSE in v1. *Proof:* smoke test hits every route;
  a non-loopback `POST /runs` is refused.
- **FR-19** Embedded dashboard (dependency-free, `go:embed`) renders these screens from the API,
  dark security-ops aesthetic, responsive, honest "untested" state visually distinct:
  - **First-class (read-only, built M7):** Posture Scorecard (landing, service × family matrix),
    Fail-Open Board, Live Run Console (polls a run), Safety Preview (renders a `runs:plan` decision
    with all 10 gates).
  - **Supporting (clearly scoped):** Experiment Builder = a hypothesis-first *compose-and-export-YAML*
    tool that validates via `experiments:validate` (it does not write to a live cluster); Game Day
    console = a read-only timeline view over a game-day resource.
  *Proof:* `make serve --demo` + a smoke test hitting each route; live browser verification in Phase 4.

**Dead-man's-switch**
- **FR-21** The capability handed to a detonator carries a hard TTL. The agent arms a dead-man's
  switch that reverts the injection if the control plane stops heart-beating (configurable, default
  10s of silence) or the hard deadline (`duration + 20%`) passes — *without* the control plane. This
  is why the platform cannot cause an incident by being down: the fault self-heals. *Proof:* a test
  injects via the mock environment, then simulates control-plane death (stop heartbeats); the mock
  environment asserts the condition was reverted after the TTL, with reason `control_plane_unreachable`.

**Operator (separate module)**
- **FR-20** CRDs `SecurityChaosExperiment` + `SecurityChaosRun` (group `chaos.shoebill.dev`,
  `v1alpha1`) with valid OpenAPI schema; a controller skeleton that reconciles a Run by delegating
  to the control plane's Safety Governor (`Plan` then `Execute`), never injecting directly. *Proof:*
  a core-module test (`internal/crd`) validates every CRD manifest (apiVersion/kind/group/version/
  served/storage/schema + both kinds). The operator is a **separate module and scaffolding**: it is
  not built in this repo's CI and a full fake-client reconcile test requires the operator's
  cluster-dev toolchain (controller-runtime + envtest), so it is **deferred** and documented in
  `operator/README.md` — not claimed as done.

## 3a. Core definitions (so a stranger can build M2/M6 unambiguously)

**The 9 experiment families** (each ships ≥4 experiments in M6):
1. `mesh` — Service Mesh Identity & Authorization (mTLS, authz policy, workload identity, egress).
2. `gateway` — Gateway & Edge Controls (authn on session-service degradation, rate limits, JWT
   validation, WAF, bot management, tenant isolation, direct-to-service bypass).
3. `identity` — Identity, Secrets & Cryptographic Lifecycle (authz fail-closed, revocation TTL,
   credential rotation, cert expiry/trust, KMS regional failover, leaked-credential detection).
4. `network` — Network & Perimeter (network-group isolation, DNS integrity, firewall fail-closed,
   QoS starvation of security traffic, cloud-metadata access T1552.005).
5. `data` — Data Protection & Privacy (tokenization bypass on vault degradation → raw PII =
   FAIL_OPEN, canary-token boundary, CDC leakage, dataset access, log redaction).
6. `supplychain` — Supply Chain & Deployment (unsigned-image admission, privileged-pod admission,
   pipeline tamper, secret-in-repo gate, dependency confusion).
7. `dr` — DC/DR & Resilience–Security Intersection (posture survives failover, failover doesn't
   widen access, break-glass hygiene, **every degradation mode preserves authz**, config locality).
8. `detection` — Detection & Response Efficacy (detection-rule regression, alert-to-page path,
   runbook executability, log-pipeline integrity, alert-fatigue measurement).
9. `ai` — AI / Agentic Surface (prompt-injection resistance, agent identity/authz, MCP trust
   boundary, memory poisoning, agent data-egress, model-endpoint authz) — mapped to OWASP LLM/ASI.

**Ladder-stage weights** (sum to 1.0 over the full ladder; the raw score normalizes by the weight
of the stages an experiment actually asserts): `PREVENTED 0.40 · DETECTED 0.25 · ALERTED 0.20 ·
PAGED 0.10 · TICKETED 0.05`. A PASS contributes full weight, SLOW half, FAIL/SKIP/INCONCLUSIVE zero.

**Service tier & the expected-control set** (the inputs FR-11's decay + cap depend on):
- **Tier** is a required property of the *target service* (`target.service.tier`), an integer
  criticality: **1 = most critical** (customer/user-path or regulated, e.g. payments), **2 =
  important** (significant but not user-path), **3 = standard**. "Tier-1" throughout this spec means
  `tier == 1`. Freshness decay uses τ=14 days for Tier-1 and τ=30 for Tier-2/3; the fail-open cap
  (service score ≤ 0.4) applies when an open FAIL_OPEN finding exists on a Tier-1 service. An
  experiment definition is *not* tiered; the tier comes from the service it is bound to.
- **Expected-control set:** each service has a set of *expected* `(control, family)` bindings it
  should pass — sourced from a **registry provider** (injectable; the demo seeds it from a
  `services.yaml`). The service-score denominator is this expected set, so a control that is
  *expected but never tested* scores `0.0` (untested ≠ unknown). Beyond the demo seed, the registry
  provider is the integration point for a real service catalog.

**The 10 Safety Governor gates** — evaluated *in order, all evaluated* (never short-circuited); each
returns a refusal with a human reason + a remedy. In the offline core, incident/registry state comes
from injectable providers (mock in demo/tests; real adapters later). Default promotion window N = 14
days.

| # | Gate | Input source | Pass condition | Refusal remedy |
|---|------|--------------|----------------|----------------|
| 1 | `KILLSWITCH` | in-memory flag + watched config | flag inactive | "Kill switch is engaged; clear it to run." |
| 2 | `SEV_ACTIVE` | incident provider (mock/webhook) | no open SEV-0/1 for the target team | "Active incident in <team>; wait until resolved." |
| 3 | `BLACKOUT` | blackout calendar (data) | now ∉ any freeze window for env/team | shows next eligible time |
| 4 | `PROMOTION` | run history store | experiment PASSed in every lower env within N days | "Run in <env> first (last pass > Nd)." |
| 5 | `CAPABILITY` | requested caps vs granted caps | prod / payments-path / AI-path caps present when required | "Request capability <x> from AppSec." |
| 6 | `APPROVAL` | run.Approvals | env policy met (prod: ≥2 distinct-team approvals) | "Need N approvals from distinct teams." |
| 7 | `BUDGET` | scope vector vs policy (native engine) | scope ⊆ env envelope ∩ experiment BlastRadius | shows which dimension exceeded + the max |
| 8 | `FAULT_DOMAIN` | target.FaultDomainID | distinct from other in-flight runs' domains (when required) | "Targets share fault domain <id>." |
| 9 | `CONCURRENCY` | in-flight run semaphores | free slot at global/env/team/service scope | "N of M slots in use; retry later." |
| 10 | `STEADY_STATE` | run the experiment's steady-state probes *now* | all probes hold | "System not in steady state: <probe>." |

**Capability** (the object FR-2/FR-7 hinge on): `{id, runUUID, scopes[], issuedAt, expiresAt,
signature}` signed by the governor's key. `Authorize(spec)` checks: signature valid against the
governor public key, `now < expiresAt`, and the detonator's requested action ⊆ `scopes`. A detonator
with no/expired/mis-scoped/forged capability refuses.

## 4. Architecture

```
                         ┌─────────────────────────────────────────────┐
                         │                 shoebill CLI                 │
                         │  catalog · run plan/exec · score · evidence  │
                         │          serve · demo · version              │
                         └───────────────┬───────────────┬─────────────┘
                                         │               │
             ┌───────────────────────────┘               └───────────────┐
             ▼                                                            ▼
   ┌──────────────────┐        ┌──────────────────────────┐     ┌─────────────────┐
   │   REST API (api) │◀──────▶│      Engine (engine)      │     │  Web dashboard  │
   │  scores/runs/... │        │   the 7-step core loop    │     │ (ui, go:embed)  │
   └──────────────────┘        └───┬───────┬───────┬───────┘     └─────────────────┘
          ▲                        │       │       │
          │ store                  ▼       ▼       ▼
   ┌──────────────┐        ┌─────────┐ ┌────────┐ ┌──────────┐
   │ store (mem/  │        │ safety  │ │detonator│ │  verify  │
   │  json file)  │        │governor │ │interface│ │  engine  │
   └──────────────┘        └────┬────┘ └───┬────┘ └────┬─────┘
                                │          │           │
                        policy(data)   mock/http/   sources: prometheus/
                        + gates        exec/sim/    alertmanager/http/mock
                                       k8s(op)      │
                                                    ▼
                                            evidence (Ed25519,
                                            hash-chained ledger)
                                                    │
                                                    ▼
                                            score (efficacy + decay)
                                                    │
                                                    ▼
                                            findings (lifecycle)
```

**Key interfaces**
- `Detonator` — `Plan(spec) / Detonate(ctx, spec, Capability) / Revert(ctx, spec, Capability) /
  VerifyReverted(ctx, spec) / Fingerprints(uuid)`. `Detonate`/`Revert` refuse unless the Capability
  authorizes the action (signed by the governor, unexpired, scope ⊇ action). This makes the governor
  the only door to injection *structurally*, not by convention.
- `SignalSource` — `Query(ctx, q) (Observation, error)`; implementations: prometheus, alertmanager,
  http, mock. A source error surfaces as `INCONCLUSIVE`, never PASS.
- `SafetyGovernor` — `Evaluate(ctx, RunRequest) (Decision, error)`; `Decision{Allowed, Refusals[],
  Constraints, Capability, PolicyVersion, PreflightProof}`.
- `Store` — experiments/runs/findings/receipts persistence; `memory` + `jsonfile` impls.
- `Clock` and `UUIDGen` are injected everywhere for determinism/testability.

**Trust boundaries / untrusted input:** experiment YAML (validated before use), signal-source
responses (parsed defensively; errors → INCONCLUSIVE), API requests (JSON-schema-validated), and —
in the operator — CRDs from the cluster. The evidence signing key is the one secret; the public key
verifies receipts. No real secrets/PII ever enter an experiment.

**Tech choices + rationale**
- **Go** — K8s-native ecosystem (client-go, controller-runtime), single static binary, Ed25519 in
  stdlib, matches the detonators we integrate (Stratus/Threatest are Go). Rejected: Python (weaker
  single-binary story, cgo-free SQLite friction), TypeScript (awkward for CRDs/operator).
- **Runtime deps (core), each justified & minimal:** `spf13/cobra` (CLI UX, ubiquitous),
  `gopkg.in/yaml.v3` (catalog). That's it for the core. **Ed25519**: stdlib. **Storage**: stdlib
  (in-memory + JSON file) — no DB dependency for the core; a SQLite/Postgres adapter is a later,
  optional add behind the `Store` interface. **Policy**: native Go engine reading policy data (no
  OPA dependency by default; OPA/Rego documented as a pluggable alternative). **Web UI**:
  dependency-free vanilla HTML/CSS/JS embedded via `go:embed` — no Node runtime, no CDN, works
  offline (enterprise requirement). Rejected heavy deps: OPA-as-library (hundreds of transitive
  deps; offline native engine is more testable), a SPA framework (build/runtime weight vs. benefit).
- **Operator module** is nested (`operator/` with its own `go.mod`) so controller-runtime/client-go's
  large dependency tree never burdens the core build. The core builds and tests with 2 direct deps.
- **Build/test:** `make`, standard `go test`, `go vet`; `staticcheck` optional.

## 5. Milestones (build order — runnable early, each independently verifiable)

- **M1 — Domain model + evidence.** `internal/model` (verdict, ladder, experiment, run, assertion,
  capability), `internal/evidence` (Ed25519 sign/verify, hash-chained ledger with monotonic seq +
  signed head). *Accept:* `go test ./...` green; tamper test fails verification; mid-chain
  reorder/removal fails; tail-truncation fails via head mismatch. (FR-9 types, FR-12/13/14)
- **M2 — Safety Governor.** 10 gates, all-evaluated, native blast-radius policy, suppression safety,
  dead-man's-switch contract. *Accept:* governor tests incl. multi-refusal and env envelopes.
  (FR-4..8)
- **M3 — Detonators + signal sources + verification engine.** `Detonator` interface + mock/http/exec/
  sim; `SignalSource` + mock/prometheus/alertmanager/http; adjudication. *Accept:* contract tests +
  full adjudication table. (FR-2,3,9,10)
- **M4 — Engine + scoring + findings + store.** The 7-step loop wired end-to-end; efficacy score;
  findings lifecycle; mem/json store. *Accept:* an in-process run produces a signed receipt + a
  score. (FR-1,11)
- **M5 — CLI + offline demo.** `shoebill` command tree; `demo --seed` runs the whole loop against a
  mock environment (incl. a deliberately fail-open control so a real `FAIL_OPEN` is produced) and
  writes receipts + scores. *Accept:* `make demo` prints a scorecard incl. a FAIL_OPEN; `make verify`
  validates the ledger. (FR-17)
- **M6 — Catalog.** ≥40 experiments across 9 families, each standards-mapped; `catalog validate`.
  *Accept:* `make catalog-validate` green; a malformed fixture fails. (FR-15,16)
- **M7 — REST API + embedded dashboard.** 6 screens rendering demo data. *Accept:* `make serve`;
  smoke test hits every route; live-verified in a browser (contrast, responsive, dark mode, the
  "untested" state visible). (FR-18,19)
- **M8 — CRDs + operator skeleton.** api types + CRD YAML + reconcile skeleton (nested module).
  *Accept:* CRD YAML validates; reconcile unit test with a fake client. (FR-20)

## 6. Test strategy

- **Unit (offline, deterministic):** every FR above has a named test. Clock + UUID injected. No
  network/FS in unit tests (mock detonator + mock source). Target ≥ 80% coverage on `internal/safety`,
  `internal/verify`, `internal/evidence`, `internal/score` (the correctness-critical packages).
- **Property/table tests:** the full adjudication matrix; the scoring formula across staleness and
  the FAIL_OPEN cap; the governor across all gate combinations.
- **Contract tests:** every `Detonator` satisfies the UUID-fingerprint + idempotent-revert contract;
  every `SignalSource` surfaces errors as INCONCLUSIVE.
- **End-to-end:** the demo loop as a test (`engine` drives mock env → receipt → verify → score).
- **Adversarial (Phase 5):** attack-tests that try to bypass the governor, forge a receipt, suppress
  an asserted alert, or downgrade INCONCLUSIVE/FAIL_OPEN — each must fail to do so, and each becomes a
  permanent regression test.
- **"Self-checks clean"** = `make test` green, `go vet` clean, `make demo` reproduces a FAIL_OPEN,
  `make verify` validates the ledger, and every README claim has a command that demonstrates it.

## 7. Docs, site & deck plan (executed Phase 4)

- **README:** one-line pitch → the gap (from finding.md) → 60-second quickstart (`go install` / `make
  demo`) → a screenshot of the scorecard → the core loop diagram → links. Doubles as the pitch deck.
- **docs/**: `architecture.md`, `safety-model.md` (the 10 gates + dead-man's-switch + why it can't
  cause an incident), `standards-mapping.md` (every experiment → ATT&CK/OWASP/compliance IDs),
  `can-and-cannot.md` (honest limits), `writing-experiments.md`, `eli5.md` (non-technical).
- **Site (GitHub Pages):** Home (animated hero, core-loop motion, copy-install), ELI5, Docs,
  Architecture, Contact — dark security-ops aesthetic, scroll-driven reveals, 60fps transform/opacity
  animation, all gated behind `prefers-reduced-motion`. Built via `master-designer` in Phase 4.
- **Deck:** README-embedded deck + social-preview card + a demo GIF of the scorecard and a run.

## 8. Honest limits (written before building)

- It verifies **behavior at the moment of the run**, not continuously between runs — hence the
  freshness decay; "green" means *recently* proven, not permanently.
- A `PASS` means "no violation observed by the configured oracles", not a proof of correctness for
  all inputs. Honest labeling, not a guarantee.
- Coverage is only as good as the catalog and the signal sources wired up; an unobserved stage is
  `INCONCLUSIVE`, surfaced, not hidden.
- In this first release the real detonators that touch infrastructure (k8s/istio/network) live in
  the operator module and require a cluster; the **core + demo are fully exercised offline** with
  mock/http/exec detonators. The platform does not ship exploits; it cannot test a control it has no
  detonator or signal source for.
- **Evidence ledger scope:** the ledger is *tamper-evident and reorder/truncation-evident* to anyone
  holding the public key — it is **not** protection against a compromised signing key (a key holder
  can rewrite and re-sign the whole chain + head). Strong non-repudiation needs an external witness /
  transparency log, which is out of scope for v1. The signing key is the one secret; production key
  management (HSM/KMS, rotation) is the operator's responsibility and is documented, not implemented.
- **Demo signing key:** `shoebill demo` generates a fresh, ephemeral keypair under `.shoebill/` — it
  is a throwaway for local reproducibility, never a trusted root. No private key is committed to the
  repo. In v1 a **single Ed25519 key** signs both governor capabilities and evidence receipts (this
  is the "one secret"); production deployments may split them and is documented, not implemented.
- **"Cannot cause an incident by being down" is demonstrated in the mock environment in v1.** The
  dead-man's-switch (FR-21) and the capability gate are fully exercised against the in-process mock;
  the real infrastructure detonators + agent that would enforce it on a live cluster live in the
  operator skeleton (FR-20) and are not end-to-end verified here.
- **Game Day is read-only in v1:** a game-day resource groups a sequence of runs for a pre-event
  gate; the console renders its timeline and readiness. Authoring/launching a game day from the UI is
  deferred; it is composed via the CLI/catalog in v1.
- **Charter enforcement is specific, not magic:** tests assert what is mechanically checkable —
  inject-mode experiments declare `reversible: true`, scope ⊆ blast-radius bounds, suppression ∩
  assertions = ∅, no assertion reads a field named like a real secret/PII (canary-tokens only). They
  cannot prove the absence of all possible misuse; the Charter is also a social contract.
- It is not a substitute for red teaming, pen testing, or static analysis; it complements them.

## 9. Definition of Done (Phase 3 exit)

- [ ] FR-1..FR-21 (including FR-11a and the FR-21 dead-man's-switch) implemented or explicitly
  deferred with a reason in `BUILD_LOG.md`.
- [ ] `make build`, `make test`, `go vet` all clean; coverage targets met on the 4 critical packages.
- [ ] `make demo` runs the whole loop offline and produces a real `FAIL_OPEN` finding + a scorecard.
- [ ] `make verify` validates the signed ledger; a tamper test proves verification catches edits.
- [ ] `make catalog-validate` passes for ≥40 standards-mapped experiments across 9 families.
- [ ] `make serve` serves the 6-screen dashboard from demo data, live-verified in a browser.
- [ ] CRDs validate (core-module `internal/crd` test). Operator is scaffolding; its full reconcile
  test is deferred to a cluster-dev env (documented, not claimed as done).
- [ ] README + docs present; every README claim is reproducible by a shipped command.
- [ ] `HANDOFF.md` lists everything only the user can do.
