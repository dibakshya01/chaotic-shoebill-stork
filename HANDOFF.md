# Handoff

The project is **built, hardened, pushed, released, and live**. This file records what was done for
you and the few remaining steps that need your account/decisions.

> Local toolchain note: this repo was bootstrapped with Go installed at `~/.local/go-sdk` (it was not
> on the system before) and a `goreleaser` at `~/go/bin`. On your own machine, use your own Go 1.24+.

## ✅ Already done

1. **Code** — full control plane (Safety Governor, verification engine, Ed25519 evidence ledger,
   efficacy score, engine, dead-man's switch), CLI, offline demo, REST API + embedded dashboard,
   60-experiment catalog, Kubernetes CRDs + controller skeleton. `go test ./... -race` green.
2. **Pushed** — github.com/dibakshya01/chaotic-shoebill-stork, branch `main` (confidential source doc
   never committed; zero internal names in the tree — audited).
3. **CI green** — the CI workflow passed on `main` (gofmt, vet, `go test -race`, coverage, catalog
   validate, demo + evidence verify).
4. **Landing site LIVE** → https://dibakshya01.github.io/chaotic-shoebill-stork/ (GitHub Pages,
   `/docs`). Animated, responsive, reduced-motion-safe, with the Security Chaos Engineering explainer.
5. **Release v0.1.0 published** — cross-platform binaries (linux/darwin/windows × amd64/arm64) +
   checksums: https://github.com/dibakshya01/chaotic-shoebill-stork/releases/tag/v0.1.0
6. **Container image** building/pushed to `ghcr.io/dibakshya01/shoebill:v0.1.0` (multi-arch, cosign
   keyless-signed by the release workflow).
7. **Hardened** — 3 back-to-back adversarial review rounds; all findings fixed (see `BUILD_LOG.md`).

## ✅ Also done (steps 1–3)

1. **GHCR container image is PUBLIC** — it inherited public visibility from the public repo
   (confirmed: "This package is currently public"). `docker pull ghcr.io/dibakshya01/shoebill:v0.1.0`
   works for everyone.
2. **Repo polished** — description + homepage set; 9 topics (security-chaos-engineering,
   chaos-engineering, security, appsec, kubernetes, golang, devsecops, security-tools, sre); Issues +
   Discussions enabled; **private vulnerability reporting enabled**.
3. **Branch protection on `main`** — requires the `build · vet · test` CI check (strict/up-to-date),
   blocks force-pushes and deletion, linear history; admins not enforced so you (solo maintainer)
   aren't locked out. Add a required review once you have collaborators.

## ⏭ What still needs you

4. **Homebrew tap** — the release pipeline is already wired (`homebrew_casks` in
   `.goreleaser.yaml`; the token is read from the `HOMEBREW_TAP_GITHUB_TOKEN` secret). Three
   steps, all needing your GitHub account, then it's automatic on every tag:
   1. Create a public repo **`dibakshya01/homebrew-tap`** (empty is fine).
   2. Create a **fine-grained PAT** scoped to just that repo with **Contents: Read and write**
      (`https://github.com/settings/personal-access-tokens/new`). Classic PAT with `repo` also works.
   3. Add it as an **Actions secret** named **`HOMEBREW_TAP_GITHUB_TOKEN`** on
      `chaotic-shoebill-stork` (Settings → Secrets and variables → Actions → New repository secret).
   Then cut the next tag (`git tag v0.1.1 && git push origin v0.1.1`) — GoReleaser commits
   `Casks/shoebill.rb` to the tap, and users run `brew install dibakshya01/tap/shoebill`.
   ⚠️ Set the secret **before** tagging, or the release's brew step fails (re-runnable).
   Meanwhile `go install` already works:
   `go install github.com/dibakshya01/chaotic-shoebill-stork/cmd/shoebill@latest`.
5. **Launch** (your accounts, your voice): draft copy is in `LAUNCH.md` (git-ignored, never
   committed) — a Show HN, a Reddit r/netsec / r/kubernetes post, and a short social post.
6. **GitHub Marketplace does NOT apply** — it lists only Actions/Apps; this is a CLI/platform.
   Distribution is `go install` + GHCR container + Homebrew + K8s.

## Next engineering milestone (see `ROADMAP.md`)

Complete the Kubernetes operator (real client-go detonators + the autonomous agent), then cloud
attack coverage via Stratus Red Team, then an external evidence witness/transparency log.
