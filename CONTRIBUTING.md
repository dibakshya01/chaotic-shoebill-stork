# Contributing

Thanks for considering a contribution. This project has a strong [Charter](CHARTER.md) and a set of
load-bearing invariants in [AGENTS.md](AGENTS.md) — please read both first.

## Getting started

```bash
git clone https://github.com/dibakshya01/chaotic-shoebill-stork
cd chaotic-shoebill-stork
make build      # build the shoebill binary
make test       # all unit tests (offline, deterministic)
make demo       # run the full loop in-process
make lint       # go vet (+ staticcheck if installed)
```

Requires Go 1.24+.

## Ground rules

- **The Safety Governor is the only door to injection.** Never add a code path that detonates without
  a governor-minted capability. Tests enforce this; keep them green.
- **Never weaken the honest verdicts.** `FAIL_OPEN` dominates; `INCONCLUSIVE` never becomes `PASS`;
  untested scores `0.0`. If your change touches `internal/safety`, `internal/verify`,
  `internal/score`, or `internal/evidence`, add a test for the invariant you touched (these paths
  are CODEOWNERS-gated).
- **No network or filesystem in unit tests.** Use the `mock` detonator and `mock` source.
- **Experiments are data.** New catalog experiments go under `catalog/<family>/` and must pass
  `shoebill catalog validate ./catalog`. See [docs/writing-experiments.md](docs/writing-experiments.md).
- **Keep runtime dependencies minimal and justified.** The core module builds with two direct deps.

## Pull requests

1. Open an issue first for anything non-trivial.
2. Keep PRs focused; include tests and update docs.
3. Run `make test` and `make lint` locally; CI runs them on every PR.
4. Describe the change and its safety implications in the PR body.

## Reporting security issues

Do **not** open a public issue for a vulnerability — follow [SECURITY.md](SECURITY.md).

## Code of Conduct

Participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).
