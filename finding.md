# Findings — chaotic-shoebill-stork

> A clean-room, vendor-neutral research memo for an open-source **Security Chaos Engineering**
> platform. Research compiled 2026-10-12 from primary sources (OWASP, MITRE, NIST, CNCF, vendor
> repos) via two parallel research passes. Sources are listed at the end; version/date-sensitive
> claims are dated inline.

## 1. Problem

Organizations deploy world-class security *controls* — service-mesh mTLS and authorization policy,
API-gateway authentication and rate limits, secrets/cert lifecycle, network policy, PII
tokenization, image-admission control, detection rules and paging. What almost none of them have is
**continuous, automated proof that any of those controls still work.**

Control efficacy is discovered in one of two expensive ways:

- **At 3 a.m. during an incident**, when a control that was configured correctly last quarter has
  silently decayed — a mesh policy reverted to permissive during a migration, an allowlist widened
  for a debugging session and never narrowed, a detection rule that references a metric label a
  dependency renamed, an authorization service whose timeout fallback happens to be *permissive*.
- **Annually during an audit**, which samples *configuration* (intent) rather than *behavior*.

Three failure classes are structurally invisible today:

- **Class A — Silent control decay.** A control worked, then stopped working, and nothing broke, so
  no alert fired and no ticket exists.
- **Class B — Fail-open under stress.** The control works in steady state and degrades
  *permissively* under load or partial failure. This is the direct, under-tested cost of the
  graceful-degradation patterns that modern availability engineering depends on: every fallback
  path was written and tested for *availability*, almost none for *authorization*. Highest severity,
  lowest observability.
- **Class C — Emergent cross-domain gaps.** No single team owns the seam between reliability and
  security (does a DC failover carry authorization state with it? does PII tokenization still hold
  when the network is saturated and the control's traffic is in a starvable QoS class?).

**Who feels it:** AppSec / product-security engineers (own control efficacy, whose only feedback
loop today is incidents), detection engineers (rules written once, never re-tested), SREs (chaos
drills test availability but never security), and GRC (manual evidence collection consuming
engineer-weeks per audit cycle). **Cost of the status quo** is tail risk: a single fail-open auth or
tokenization incident on a critical path can dwarf years of a platform's cost, and every quarter of
delay carries that risk into the next peak-traffic event.

## 2. Prior art & the gap

Reliability chaos engineering is mature and well-tooled. **Security** chaos engineering — proving
*security controls* still behave under a safe, governed, adversarial condition — is essentially
un-served in open source. (Full table; stars/dates pulled live 2026-10-12.)

| Tool | What it does | Strengths | Gaps for *security-control efficacy* | License |
|------|--------------|-----------|--------------------------------------|---------|
| **ChaoSlingr** | First OSS security-chaos tool; random security faults on AWS | Historic proof that controls silently fail | **Archived since 2019**, AWS-only, no assert loop / scorecard / governor / evidence (69★, dead) | Apache-2.0 |
| **Verica** (comm.) | "Continuous verification" (availability + security) | Pedigree (ChaoSlingr + Netflix) | Closed-source; no public control-efficacy-scorecard product | Commercial |
| **SafeBreach / AttackIQ / Picus / Cymulate** (comm. BAS) | Continuous control validation + coverage scorecards | **Closest to the target problem** | **All closed-source**; opaque; **no signed-evidence/attestation story** — the OSS whitespace | Commercial |
| **LitmusChaos / Chaos Mesh** | CNCF K8s-native fault injection | Mature governor, CRDs, probes — ideal **detonator engines** | Reliability faults only; no security semantics, no detect/alert/page assertion | Apache-2.0 |
| **Chaos Toolkit** | Provider-agnostic experiment spec | "Steady-state hypothesis" model maps to control-hypotheses | Reliability framing; security is DIY; no fleet governor/evidence/scorecard | Apache-2.0 |
| **Gremlin / Steadybit / AWS FIS / Azure Chaos Studio** | Commercial/managed reliability chaos | Blast-radius controls, reliability scorecards worth studying | Reliability-only; no attacker TTPs or detection assertion | Commercial |
| **Stratus Red Team** (Datadog) | "Terraform for attacks": cloud/K8s ATT&CK techniques, self-cleaning | Excellent **adversary detonator**, Go-importable, MITRE-mapped | Detonator only — no assertion a control caught it, no scorecard/governor | Apache-2.0 |
| **Atomic Red Team** (Red Canary) | 200+ ATT&CK "atomic" host tests | De-facto endpoint **detonator** library | Reports exec success, not whether detection fired; not for unattended prod | MIT |
| **Caldera** (Apache) | Multi-step adversary emulation | Full attack-chain orchestration | Heavyweight; emulation only; no SLO-bounded control assertion | Apache-2.0 |
| **Threatest** (Datadog) | Go framework: detonate → expect alert, **UUID-correlated** | **The key detect-then-assert primitive** | Library not platform; Datadog-signal-centric; no governor/scorecard/evidence | Apache-2.0 |
| **Falco** (CNCF, graduated) | Runtime syscall/K8s-audit detection | Production-grade — ideal **signal source** for DETECT/ALERT | Detection engine, not a tester | Apache-2.0 |
| **Sigma** | Vendor-neutral detection-rule format | Standard for detection-as-code | A rule format + signal, not a tester | DRL |
| **kube-bench / Trivy / Kubescape / Checkov** | Static posture / IaC / SBOM scanning | Ubiquitous, broad static coverage | **Static, point-in-time** — never verify a control *behaves under attack* | Apache-2.0 |

**The gap we fill (one paragraph, honest):** There is clean, specific open-source whitespace for a
"chaos engineering **for security controls**" platform whose differentiators are a first-class
**Safety Governor** and **signed evidence + per-service control-efficacy scorecards**, wrapping an
attack-and-assert loop (detonate a bounded adversarial condition → assert PREVENTED / DETECTED /
ALERTED / PAGED within an SLO → sign the result → score it). Today that composition exists only
inside closed commercial BAS tools, or as a long-dead 2019 prototype. Every open-source project owns
only a *slice*: reliability-chaos tools have governors but no security semantics; adversary tools
detonate but don't assert efficacy or score; Threatest asserts but is a single-signal library with
no governor, fleet orchestration, or evidence. **We build the missing composition and integrate the
slices.**

## 3. Technical landscape

**The standard shape of "good" is an attack-and-assert loop with a correlation key.** The canonical
pattern (popularized by Datadog's Threatest) assigns every detonation a UUID, weaves it into every
artifact the detonation produces, and *requires* that UUID when matching downstream signals — so a
run that merely coincides with unrelated alert noise can never score a false PASS. We adopt this as a
mandatory invariant.

**Build vs. borrow.** We build the orchestration + governor + verification + evidence + scorecard
layer and integrate everything below rather than reinvent it:

- **Reliability/fault detonators:** LitmusChaos / Chaos Mesh fault primitives (pod/container kill,
  network delay/partition, DNS). CRD model (reusable *definition* vs. bound *intent*) borrowed from
  their proven separation.
- **Adversary detonators:** Stratus Red Team (Go-importable), Atomic Red Team (host atomics),
  Caldera (multi-step) — all behind one `Detonator` interface.
- **Policy / safety:** OPA / Rego (and Kyverno for K8s admission) to express blast-radius and
  blackout rules as reviewable, diffable, unit-testable *data* owned by the security team — not
  `if` statements owned by the platform author. (We default to a zero-dependency native policy
  engine so the governor works offline, with OPA as a documented pluggable alternative.)
- **Signal sources:** Prometheus + Alertmanager (steady-state, DETECTED, ALERTED), paging APIs
  (PAGED), Falco + Sigma-driven SIEM (DETECTED), the control's own enforcement path (PREVENTED).
- **Evidence:** signed, self-describing receipts. We use **Ed25519** (public-key verifiable so an
  auditor needs no secret) and leave room for Sigstore/cosign and in-toto/SLSA attestation envelopes.
- **Hypothesis source:** decision-tree thinking (e.g. Deciduous) to choose *which* controls to test.

**The hard parts** (where this project earns its keep): (1) a Safety Governor that is the *only* path
to injection and is itself incapable of causing an incident; (2) adjudication that never lies —
`FAIL_OPEN` is its own verdict, `INCONCLUSIVE` never becomes `PASS`; (3) a scorecard that treats
*untested* as `0.0`, not "unknown", and decays with staleness so green means *currently* proven.

## 4. Standards & references

Every experiment maps to authoritative IDs for credibility and audit evidence. Current versions
verified 2026-10-12.

- **MITRE ATT&CK v19** (v19.0 released 2026-04-28; v19.2 is the current published version as of
  2026-10; the *Defense Evasion* tactic was split into **Stealth** + **Defense Impairment** —
  technique IDs are stable). Matrices: Enterprise, Cloud
  (IaaS/Identity), Containers (Kubernetes). Example techniques we map to: T1078 Valid Accounts,
  T1550 Alternate Auth Material, T1552.005 Cloud Instance Metadata API, T1557 AiTM, T1562 Impair
  Defenses, T1610/T1611/T1613 container techniques, T1530 Data from Cloud Storage, T1567
  Exfiltration Over Web Service. Companion **MITRE ATLAS** for the AI family.
- **OWASP API Security Top 10 (2023)** — API1 BOLA, API2 Broken Auth, API5 BFLA, API8 Misconfig,
  API10 Unsafe Consumption. **OWASP Top 10 (Web) 2025** — A01 Broken Access Control, A03 Software
  Supply Chain Failures (new), A09 Security Logging & Alerting Failures.
- **OWASP Top 10 for LLM Applications 2025 (v2.0)** — LLM01 Prompt Injection, LLM02 Sensitive Info
  Disclosure, LLM06 Excessive Agency, LLM07 System-Prompt Leakage. **OWASP Top 10 for Agentic
  Applications 2026 (ASI01–ASI10)** — ASI01 Goal Hijack, ASI02 Tool Misuse, ASI03 Identity &
  Privilege Abuse, ASI06 Memory & Context Poisoning. **OWASP AISVS v1.0/1.01 (2026)** — C5 Access
  Control for AI, C9 Orchestration & Agentic, **C10 MCP Security**. **OWASP MCP Top 10 (beta)**.
- **Compliance families:** ISO/IEC 27001:2022 Annex A (A.8.5 secure auth, A.8.24 cryptography,
  A.8.11 data masking, A.8.15/8.16 logging & monitoring); PCI-DSS v4.0.1 (Req 3.5 tokenization,
  Req 7/8 access & auth, Req 10 logging); NIST SP 800-53 r5.2.0 (AC/IA/SC/AU/IR/CM/CP); NIST CSF 2.0
  (PR.AA, PR.DS, DE, RS, RC); SOC 2 TSC (CC6/CC7/CC8); India DPDP Act 2023 + Rules 2025 (§8(5)
  safeguards naming virtual tokens/encryption/masking).
- **SCE canon:** Shortridge & Rinehart, *Security Chaos Engineering* (O'Reilly, 2023); CNCF
  Cloud-Native Security Whitepaper v2 (2022); NIST SP 800-160 Vol.2 (resilience).

A clean automated check means "no violation *detected*", **not** "control proven present". The repo
ships a "what it can & cannot do" doc and labels every verdict honestly.

## 5. Risks & unknowns

- **Legal / naming.** `chaotic-shoebill-stork` is unclaimed on npm, PyPI, crates.io, and GitHub
  (only the user's repo). The bare word *shoebill* is taken on package registries and crowded on
  GitHub (including a crypto wallet) — a non-issue for a Go binary distributed via `go install` /
  Homebrew tap / container, but we avoid claiming `shoebill` on language registries. License:
  Apache-2.0 (patent grant, enterprise-friendly).
- **Dual-use.** A tool that detonates adversarial conditions could be misused. Mitigated
  *structurally*: the Safety Governor is the only path to injection; capabilities are per-run,
  scoped, time-boxed, signed; the catalog is declarative hypotheses, not payloads; the Charter is in
  the repo and enforced by tests. Injection is off by default.
- **Technical.** Verification correctness is the whole product — a false PASS on a security control
  is worse than no platform. Mitigated by the UUID-correlation invariant and the
  `INCONCLUSIVE`-never-`PASS` / `FAIL_OPEN`-dominates adjudication rules, each unit-tested.
- **Adoption.** "Another scanner that produces a backlog nobody fixes." Mitigated by launching as a
  *scorecard* with auto-resolved owners and a CI gate, not a scanner; by a 5-minute offline demo
  that runs the whole loop on a laptop; and by read-only drift-detection experiments that are safe
  to run fleet-wide from day one.
- **Scope creep into red teaming.** Bounded by the Charter: anything requiring an exploit, lateral
  movement, persistence, or real data exfiltration is out.

## 6. Synthesized direction

**Thesis (one sentence):** Treat every security control as a testable hypothesis — safely detonate a
bounded, reversible, pre-declared adversarial condition; assert whether the control *prevented,
detected, alerted, and paged* within its SLO; sign the evidence; and roll it into a per-service
control-efficacy scorecard that decays with staleness.

**The wedge:** a first-class **Safety Governor** + **signed, independently verifiable evidence** +
an honest **efficacy scorecard** — the exact combination that exists today only in closed commercial
BAS tools or a dead 2019 prototype. Integrate the detonators and signal sources that already exist;
build the governed loop that doesn't.

**Guiding principles (lean on these as strengths):**
1. **One door to injection.** The Safety Governor is the only path; nothing detonates without an
   `Allowed` decision. It is architecturally incapable of causing an incident by being down.
2. **Never lie.** `FAIL_OPEN` is its own verdict; `INCONCLUSIVE` never becomes `PASS`; *untested*
   scores `0.0`, not "unknown".
3. **Causality, not coincidence.** A mandatory run-UUID woven into every artifact and required by
   every assertion.
4. **Evidence an auditor can trust without trusting us.** Ed25519-signed, self-describing,
   tamper/reorder/truncation-evident receipts (a signed hash chain with a signed head).
5. **Offline, deterministic, free, auditable.** The whole loop runs on a laptop with zero external
   dependencies; given the same inputs it yields the same verdict and a byte-stable receipt.

## Sources
(Accessed 2026-10-12. Popularity/version figures are proxies, dated inline.)

- MITRE ATT&CK versions & v19 (Apr 2026) release notes — https://attack.mitre.org/resources/versions/ · https://attack.mitre.org/resources/updates/updates-april-2026/
- OWASP Top 10 2025 — https://top10.owasp.org/2025 · API Top 10 2023 — https://owasp.org/API-Security/editions/2023/
- OWASP GenAI (LLM Top 10 2025, Agentic ASI 2026) — https://genai.owasp.org/ · AISVS — https://github.com/OWASP/AISVS · MCP Top 10 — https://owasp.org/www-project-mcp-top-10/
- NIST SP 800-53 r5.2.0 — https://csrc.nist.gov/pubs/sp/800/53/r5/upd1/final · NIST CSF 2.0 — https://www.nist.gov/cyberframework
- PCI DSS v4.0.1 — https://www.pcisecuritystandards.org/ · ISO/IEC 27001:2022 — https://www.iso.org/standard/27001 · SOC 2 TSC — https://www.aicpa-cima.com
- India DPDP Act 2023 & Rules 2025 (in force 2025-11-13)
- Security Chaos Engineering (O'Reilly, 2023) — https://www.oreilly.com/library/view/security-chaos-engineering/9781098113810 · CNCF Cloud-Native Security Whitepaper v2 — https://tag-security.cncf.io/community/resources/security-whitepaper/
- ChaoSlingr — https://github.com/Optum/ChaoSlingr (69★, archived 2019) · Threatest — https://github.com/DataDog/threatest (347★, v1.5.0 2026-09)
- LitmusChaos — https://litmuschaos.io (5,725★) · Chaos Mesh — https://www.cncf.io/projects/chaosmesh/ (7,930★) · Chaos Toolkit — https://github.com/chaostoolkit/chaostoolkit (2,030★)
- Stratus Red Team — https://github.com/DataDog/stratus-red-team (2,420★) · Atomic Red Team — https://github.com/redcanaryco/atomic-red-team (12,654★) · Caldera — https://github.com/mitre/caldera (7,415★)
- Falco — https://github.com/falcosecurity/falco (9,475★, CNCF graduated) · Sigma — https://github.com/SigmaHQ/sigma (11,219★)
- Static posture: Trivy (38,367★), Kubescape (11,791★), kube-bench (8,222★), Checkov (9,067★)
- BAS market — https://www.marketsandmarkets.com/ResearchInsight/automated-breach-attack-simulation-market.asp
- Naming (live registry checks 2026-10-12): npm/PyPI/crates/GitHub — `chaotic-shoebill-stork` unclaimed everywhere.
