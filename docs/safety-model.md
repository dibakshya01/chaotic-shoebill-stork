# The Safety Model

`shoebill` detonates adversarial conditions. The reason that is safe is not a promise — it is a set
of structural, auditable mechanisms. This document is the contract.

> See also the [Charter](../CHARTER.md) (what the platform will never do) and the
> [honest limits](can-and-cannot.md).

## One door to injection

Nothing in the platform dispatches to a detonator without a Safety Governor decision. This is
enforced two ways, not one:

1. **The engine is the only caller of `Detonate`,** and it calls the Governor's `Evaluate`
   immediately before. A denied decision returns early — no detonation, no receipt.
2. **The detonator itself refuses without a valid capability.** `Detonate(ctx, spec, capability)`
   takes a **Capability** — not a bare run id — that the Governor mints *only* on an `Allowed`
   decision. The capability is **Ed25519-signed by the Governor, scoped to one target and one
   action, and time-boxed**. Each detonator verifies the signature against the trusted Governor
   key, checks expiry, target, and scope, and refuses otherwise.

So even a caller that holds a detonator cannot inject: it has no signed capability, and it cannot
mint one (the platform cannot grant itself capabilities). This is proven by a contract test that
runs every detonator against an absent / expired / mis-scoped / forged / wrong-target capability.

## The ten gates

`Governor.Evaluate` runs ten gates **in order, all evaluated, never short-circuited**, so the caller
sees the whole envelope at once rather than fixing one gate at a time. Each refusal carries a
human-readable reason and a remedy.

| # | Gate | Refuses when… |
|---|------|---------------|
| 1 | `KILLSWITCH` | the global kill switch is engaged |
| 2 | `SEV_ACTIVE` | an incident is open for the target team (never run chaos during a real incident) |
| 3 | `BLACKOUT` | now is inside a freeze window (peak events, change freezes) |
| 4 | `PROMOTION` | the experiment hasn't PASSed in every lower environment within N days (default 14) |
| 5 | `CAPABILITY` | a required grant is missing (prod-injection, payments-path, ai-injection) |
| 6 | `APPROVAL` | approvals are insufficient (prod needs two, from **distinct** teams) |
| 7 | `BUDGET` | the requested scope exceeds the environment envelope ∩ the experiment's blast radius |
| 8 | `FAULT_DOMAIN` | the target shares a fault domain with an in-flight run (correlated loss) |
| 9 | `CONCURRENCY` | no slot is free at the global / env / team / service scope |
| 10 | `STEADY_STATE` | the system is not in steady state *right now* (injecting into a degraded system produces a garbage verdict and risks a real incident) |

### Blast-radius envelopes

The `BUDGET` gate evaluates policy as **data** (a native, dependency-free policy engine; OPA/Rego is
a documented pluggable alternative). The envelope tightens with environment impact:

| Environment | Pods | Duration | Traffic | Approvals |
|-------------|------|----------|---------|-----------|
| `playground` | ∞ | ≤ 3600s | synthetic only | self-serve |
| `stage` | ∞ | ≤ 900s, ≤50% replicas | synthetic only | 1 |
| `preprod` | ∞ | ≤ 300s, ≤20% replicas | replayed | 2 |
| **`prod-canary`** | **1** | **≤ 60s, ≤5% replicas** | **≤ 1%** | **2 distinct teams** |
| `prod-ring` | — | ≤ 60s, ≤5% replicas | ≤ 5% | standing grant |

A **payments-path** service in production may only run `observe_only` experiments — never injection.
An inject-mode experiment that is not declared `reversible` is refused outright.

## The dead-man's switch

The property that makes the whole platform safe to run at all: the agent arms a dead-man's switch
when it injects. If the control plane stops heart-beating (default: 10s of silence) or the hard
deadline (`duration + 20%`) passes, the fault is reverted **without the control plane** — it
self-heals. This is why the platform cannot cause an incident by being down.

> In v1 this is demonstrated against the in-process mock environment. The real-infrastructure agent
> that enforces it on a live cluster lives in the [operator](../operator) module.

## Suppression safety

Alert suppression exists to mute the *expected noise* of a run. It is the one API that could be
turned into a weapon, so it is bounded:

- The suppression set can **never** intersect the alert rules the experiment asserts on
  (schema-enforced — suppressing an assertion would let a decayed control score a false PASS).
- Wildcards are forbidden.
- TTL is capped at **30 minutes**, enforced server-side.

## Capabilities

A capability is `{id, runUUID, target, scopes[], issuedAt, expiresAt, signature}`. It is minted per
run, scoped to one target and action, time-boxed to the run's duration plus a small margin, and
signed by the Governor. No credential ever outlives its run. The platform cannot widen its own
scope.

## Honesty in adjudication

Two decisions carry most of the platform's epistemic integrity, and both are unit-tested:

- **`FAIL_OPEN` is a distinct, dominating verdict.** A control that degraded *permissively* (allowed
  what it should have denied) is the worst outcome. A fail-open *observation* is recorded even when
  the run is `ABORTED`, so it is never swallowed.
- **`INCONCLUSIVE` never becomes `PASS`.** If a required signal source errored or was unavailable, we
  learned nothing. Reporting PASS would manufacture false confidence in exactly the controls people
  rely on.
