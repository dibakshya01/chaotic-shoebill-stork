# AGENTS.md — conventions, invariants, and forbidden patterns

This file is read first by both humans and AI coding assistants. It encodes the rules that keep
this platform safe and trustworthy. Treat every invariant here as load-bearing.

## Repository map

```
cmd/shoebill/            CLI entrypoint (the `shoebill` binary)
internal/model/          Core domain types: Experiment, Run, Verdict, Ladder, Receipt, Score
internal/catalog/        Load + validate the experiment catalog (YAML -> model)
internal/safety/         THE SAFETY GOVERNOR — the only path to injection
internal/detonator/      Detonator interface + safe built-in detonators (mock, http, exec, sim)
internal/verify/         Verification engine: ladder, adjudication, UUID correlation
internal/verify/sources/ Signal sources: prometheus, alertmanager, http, mock
internal/score/          Control Efficacy Score (with freshness decay)
internal/evidence/       Ed25519-signed, append-only evidence receipts + verifier
internal/findings/       Finding lifecycle (open/accept/resolve)
internal/engine/         The orchestrator that runs the 7-step core loop
internal/store/          Persistence (in-memory + file-backed JSON; pluggable)
internal/api/            REST API server
internal/ui/             Embedded web dashboard (go:embed)
catalog/                 The experiment catalog itself — DATA, not code
operator/                Kubernetes CRDs + controller (separate Go module; heavy k8s deps)
docs/                    Architecture, safety model, standards mapping, can-&-cannot
```

## Hard invariants (enforced by tests — do not weaken)

1. **One door to injection.** No code path may call a `Detonator.Detonate` without first obtaining
   a `safety.Decision` with `Allowed == true`. The engine is the only caller of `Detonate`, and it
   calls the Governor immediately before. Test: `internal/engine` asserts a detonate attempt without
   a valid decision panics/errors.
2. **All gates evaluated, never short-circuited.** `safety.Governor.Evaluate` returns *every*
   refusal, not the first, so the UI shows the whole envelope at once.
3. **`INCONCLUSIVE` never becomes `PASS`.** If a required signal source errored or was unavailable,
   the verdict is `INCONCLUSIVE`. A verification engine that reports PASS when it could not observe
   is worse than no engine.
4. **`FAIL_OPEN` dominates.** If a control permitted something it should have denied, the verdict is
   `FAIL_OPEN` regardless of what else passed.
5. **UUID correlation is mandatory by default.** Every detonator weaves the run UUID into every
   artifact. An assertion may set `requireUuid: false` only with written `justification`.
6. **Suppression can never cover an assertion.** The suppressor rejects any request whose rule set
   intersects the experiment's asserted alert rules, and caps TTL at 30 minutes.
7. **Revert is idempotent and independently verified.** `Revert` is safe to call repeatedly;
   `VerifyReverted` must use a path independent of `Revert`.
8. **The platform cannot grant itself capabilities.** Capability minting is per-run and scoped by
   the Governor decision only.

## Code conventions

- **Language:** Go. Keep files small and focused; prefer many small packages over god-packages.
- **No `panic` in library code** except to assert a violated invariant that indicates a programming
  bug (and document it). Return errors otherwise.
- **Errors wrap context** (`fmt.Errorf("...: %w", err)`); never discard an error silently.
- **No network or filesystem access in unit tests.** Use the `mock` detonator and `mock` signal
  source. The whole detonate->verify->score->sign loop must be testable offline and deterministically.
- **Determinism is a feature.** Given the same inputs, the engine must produce the same verdict and a
  byte-stable (modulo timestamps/uuid) receipt. Inject the clock and the UUID generator.
- **Security-sensitive packages (`internal/safety`, `internal/evidence`) require extra review** — see
  CODEOWNERS. Changes there must come with tests for the invariant they touch.

## Forbidden patterns

- Calling a detonator outside the engine.
- Downgrading `INCONCLUSIVE`/`FAIL_OPEN` in any aggregation or display.
- Reading real secrets/PII into an experiment. Use canary tokens.
- Adding a detonator that performs an irreversible action.
- Hardcoding environment-specific endpoints in the engine (they belong in adapters/config).

## Build & test

```
make build      # build the shoebill binary
make test       # run all unit tests (offline, deterministic)
make demo       # run the full loop in-process against the mock environment
make lint       # vet + staticcheck (if installed)
make verify     # verify the demo evidence ledger signatures
```
