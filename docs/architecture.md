# Architecture

`shoebill` is a Go control plane that runs one seven-step loop (see the
[README](../README.md#how-it-works--the-core-loop)) and integrates existing detonators and signal
sources rather than reinventing them.

## Components

```
                         ┌─────────────────────────────────────────────┐
                         │                 shoebill CLI                 │
                         │  catalog · run plan · score · evidence       │
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
   ┌──────────────┐        ┌─────────┐ ┌─────────┐ ┌──────────┐
   │ store (mem/  │        │ safety  │ │detonator│ │  verify  │
   │  json file)  │        │governor │ │interface│ │  engine  │
   └──────────────┘        └────┬────┘ └───┬─────┘ └────┬─────┘
                                │          │            │
                        policy (data)   mock/http/   sources: prometheus/
                        + 10 gates      observe/     alertmanager/http/mock
                                        k8s(operator)│
                                                     ▼
                                             evidence (Ed25519,
                                             hash-chained + signed head)
                                                     │
                                                     ▼
                                             score (efficacy + decay) → findings
```

## Key interfaces

- **`Detonator`** — `Plan / Detonate(ctx, spec, Capability) / Revert / VerifyReverted /
  Fingerprints(uuid)`. Built-ins: `mock` (offline demo/tests), `observe` (read-only, always safe),
  `http` (a single bounded request bearing the correlation header). Infrastructure detonators
  (`k8s`, `istio`, `dns`, `cert`, `network`) live in the operator module. Every detonator verifies
  the capability before acting.
- **`SignalSource`** — `Query(ctx, query, uuid) (Observation, error)`. Implementations: `prometheus`,
  `alertmanager`, `http` (real adapters), and `mock` (deterministic). **A source error surfaces as
  `INCONCLUSIVE`, never `PASS`.**
- **`Governor`** — `Evaluate(ctx, RunRequest) (Decision, error)`. The decision carries every refusal,
  the narrowed constraints, the steady-state proof, the policy version (for reproducibility), and —
  only when allowed — a signed capability.
- **`Store`** — experiments, the service registry, runs, and findings, with JSON persistence so
  `demo` can seed what `serve` reads.

## Determinism

The clock and the UUID/id generators are injected everywhere. Given the same inputs, the engine
produces the same verdict and a byte-stable receipt (modulo the injected timestamps/uuid). Unit
tests run entirely offline with the `mock` detonator and `mock` source — no network, no filesystem,
no cluster.

## Verification engine

For each assertion the engine queries the signal source, compares the `Observation` against the
`Expectation`, and applies UUID correlation (a positive signal only counts if it carries the run's
UUID, unless the assertion is a justified `requireUuid: false`). It builds the control-response
ladder — processing `DETECTED → ALERTED → PAGED → TICKETED` as a chain, so a page/ticket assertion is
`SKIP`ped rather than spuriously `INCONCLUSIVE` when the alert it depends on never fired — then
adjudicates with a total, first-match-wins truth table
(`ABORTED > FAIL_OPEN > INCONCLUSIVE > PASS > FAIL_SLOW > FAIL_UNDETECTED`).

## Control Efficacy Score

```
raw      = Σ(weight × stage_result) / Σ(weight) over the asserted ladder stages
weights  = PREVENTED 0.40 · DETECTED 0.25 · ALERTED 0.20 · PAGED 0.10 · TICKETED 0.05
freshness = e^(-days_since_last_run / τ),  τ = 14 (Tier-1), 30 (otherwise)
score     = raw × freshness        [0.0 – 1.0]
```

An expected-but-**untested** control scores `0.0` (untested ≠ unknown). An open `FAIL_OPEN` finding
on a **Tier-1** service caps that service's score at `0.4`, regardless of everything else — green must
mean *currently proven*.

## Trust boundaries

Untrusted input enters at: experiment YAML (validated before use), signal-source responses (parsed
defensively; errors → `INCONCLUSIVE`), API requests (localhost-only, JSON), and — in the operator —
CRDs from the cluster. The single secret is the signing key; its public half verifies both
capabilities and receipts. No real secrets or PII ever enter an experiment.

## Why a separate operator module

The Kubernetes integration (`operator/`) is its own Go module so its heavy
controller-runtime/client-go dependency tree never burdens the core build — the core compiles and
tests with **two** direct dependencies. The operator's reconciler delegates authorization to the
control plane's Safety Governor (`Plan` then `Execute`) rather than embedding the engine, so the
"one door to injection" invariant holds across the cluster boundary too.
