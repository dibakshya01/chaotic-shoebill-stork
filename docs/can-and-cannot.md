# What it can & cannot do

Written before the code, kept honest after. A security tool that overstates what it proves is worse
than no tool.

## What it can do

- Verify that a deployed security control behaves as hypothesized **at the moment of a run**: did it
  prevent, detect, alert, and page within its SLO?
- Produce an honest, decaying **control-efficacy scorecard** per service and family, where untested
  is visibly `0.0` and fail-open caps a Tier-1 service.
- Emit **independently verifiable, signed evidence** an auditor can check with only the public key.
- Run the entire loop **offline and deterministically** on a laptop (`shoebill demo`).
- Gate every injection through a **Safety Governor** with ten explained gates and a dead-man's
  switch.

## What it cannot do (and does not pretend to)

- **It does not prove correctness for all inputs.** A `PASS` means "no violation observed by the
  configured oracles during this run", not a guarantee. It verifies *behavior at run time*, not a
  theorem.
- **It is a snapshot, not a watch.** Between runs it knows nothing — hence the freshness decay.
  "Green" means *recently* proven, not permanently.
- **Coverage is only as good as the catalog and the wired-up signal sources.** An unobserved stage is
  `INCONCLUSIVE`, surfaced, never hidden. A control with no detonator or signal source is simply not
  tested (and scores `0.0`).
- **It is not a vulnerability scanner, red-team tool, or SIEM.** It complements SAST/DAST/SCA, pen
  testing, and your observability stack; it does not replace them. It ships **no exploits**.

## Evidence ledger — scope of the guarantee

The ledger is **tamper-evident, reorder-evident, and tail-truncation-evident** to anyone holding the
public key (a signed hash chain plus a separately-signed head). It is **not** protection against a
**compromised signing key**: a key holder can rewrite and re-sign the whole chain and head. Strong
non-repudiation needs an external witness / transparency log, which is **out of scope for v1**. The
signing key is the one secret; production key management (HSM/KMS, rotation) is the operator's
responsibility and is documented, not implemented.

The demo uses a fresh, throwaway key under `.shoebill/` (git-ignored) — never a trusted root, never
committed.

## v1 verification status (be precise)

- The control plane — Safety Governor, verification engine, scoring, evidence, CLI, API, dashboard —
  is fully built and unit-tested offline.
- The "cannot cause an incident by being down" property (capability gate + dead-man's switch) is
  **demonstrated against the in-process mock environment** in v1 (synchronous forced-revert). The
  [operator](../operator) module is now **implemented and unit-tested with fake clients** — a real
  client-go `netpol` detonator plus the reconciler that delegates to the Safety Governor — but is
  **not yet end-to-end-verified against a live cluster**, and the fully-independent agent that would
  drive the autonomous silence-revert out-of-process is still future work (see ../ROADMAP.md).
- **Game Day** is a read-only view in v1 (it groups a sequence of runs); authoring/launching a game
  day from the UI is deferred.

## Charter enforcement — what the tests actually check

The [Charter](../CHARTER.md) is partly a social contract and partly mechanical. The mechanical parts
the tests enforce: inject-mode experiments declare `reversible: true`; requested scope ⊆ blast-radius
bounds; suppression never covers an asserted alert; and the catalog uses canary-token patterns, not
real secrets or PII. Tests cannot prove the absence of all possible misuse — the Governor, the signed
audit log, and review do the rest.
