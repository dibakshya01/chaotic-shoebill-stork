# Security Policy

## This is a security tool — read the Charter first

`chaotic-shoebill-stork` detonates bounded, reversible adversarial conditions to verify that your
security controls work. Its safety model is documented in [`CHARTER.md`](CHARTER.md) and
[`docs/safety-model.md`](docs/safety-model.md). The Safety Governor is the only path to injection.

**Never point injection-mode experiments at a system you do not own or do not have written
authorization to test.** Read-only / `observe_only` experiments are safe to run widely; injection
experiments require the environment progression and approvals the Governor enforces.

## Reporting a vulnerability

If you find a security vulnerability in this project — especially anything that could let a run
bypass the Safety Governor, forge an evidence receipt, or escalate a capability — please report it
privately:

- Open a **GitHub Security Advisory** (preferred): the repository's *Security → Report a
  vulnerability* tab.
- Or email the maintainer listed in the repository profile.

Please include: a description, affected version/commit, reproduction steps, and the impact. We aim
to acknowledge within 5 business days.

**Do not** open a public issue for an unfixed vulnerability, and do not include real secrets, real
PII, or working exploits in your report.

## Supported versions

Until a 1.0 release, only the latest `main` and the most recent tagged release receive security
fixes.

## Scope

In scope: the control plane, Safety Governor, verification engine, evidence signing/verification,
CLI, API, and the embedded dashboard. Out of scope: third-party dependencies (report upstream),
and misuse of the tool against unauthorized targets (that is on the operator, and is forbidden by
the Charter).
