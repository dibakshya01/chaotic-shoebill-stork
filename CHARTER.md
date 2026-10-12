# Charter — what this platform will and will not do

`chaotic-shoebill-stork` is a **Security Chaos Engineering** platform. It verifies that the
security controls you already deployed actually work, by safely detonating bounded,
reversible, pre-declared adversarial conditions and asserting whether each control
**prevented, detected, alerted, and paged** within its stated SLO — then emitting signed
evidence.

This charter is a hard boundary, not an aspiration. The code enforces it; reviewers gate it.

## What it will do

- Treat every security control as a **testable hypothesis** with a measurable steady state.
- Run only **pre-declared, bounded, reversible** actions, every one gated by the Safety Governor.
- Weave a **correlation UUID** into every artifact so a chaos run can never be mistaken for a
  real incident, and so verification can never be a coincidence.
- Fail **closed and honest**: an unobservable result is `INCONCLUSIVE`, never `PASS`. A control
  that degraded permissively is `FAIL_OPEN`, the highest-severity verdict.
- Produce **independently verifiable, signed evidence receipts** that an auditor can check
  without trusting this platform's UI or database.

## What it will never do

The following are out of scope **by design**. They are not "later" features; they are things
this project refuses to be:

- **No exploitation of unpatched vulnerabilities / 0-days.** Blast radius is unbounded and not
  reversible. That is red-team work, under separate rules of engagement.
- **No lateral movement, persistence, or privilege-escalation campaigns.** Every action is a
  single, declared, reverted step — not a kill chain.
- **No real data exfiltration.** Ever. Data-protection experiments use uniquely fingerprinted
  **canary tokens** only, never real PII or secrets.
- **No credential cracking, phishing, or social engineering.** People are not systems under test.
- **No destructive or irreversible actions.** If an action cannot be reverted and the revert
  independently verified, the Safety Governor refuses it.
- **No "just this once" prod injection.** Production injection is off by default and requires an
  explicit, signed, time-boxed capability grant plus two-person approval from distinct teams.
- **No self-granted capability.** The platform cannot widen its own scope. Capabilities are
  minted per-run, narrowly scoped, and expire automatically.
- **No silent suppression of its own assertions.** Alert suppression is capped at 30 minutes and
  is schema-forbidden from ever covering an alert rule the experiment asserts on.

## The dual-use line

A tool that detonates adversarial conditions could be misused. The defenses against that are
structural and auditable, not a promise:

1. **The Safety Governor is the only path to injection.** Nothing in the codebase dispatches to a
   detonator without a Governor `Decision{Allowed: true}`. This is enforced by code and by test.
2. **Capability-scoped, time-boxed, signed credentials.** No credential ever outlives its run.
3. **Everything is logged and signed.** Every decision, approval, detonation, and verdict lands in
   an append-only, signed evidence ledger.
4. **The experiment catalog is data, not exploits.** Experiments declare *hypotheses and
   assertions*, not payloads with side effects.

If a proposed feature cannot be built within these lines, it does not belong here. Open an issue
and we will say so plainly.
