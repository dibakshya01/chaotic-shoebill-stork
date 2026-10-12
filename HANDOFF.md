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

## ⏭ What only you can do

1. **Make the GHCR container image public** (it defaults to private): GitHub → your profile →
   Packages → `shoebill` → Package settings → Change visibility → Public. Then
   `docker pull ghcr.io/dibakshya01/shoebill:v0.1.0` works for everyone.
2. **Repo polish**: add a description + topics (`security-chaos-engineering`, `chaos-engineering`,
   `security`, `appsec`, `kubernetes`, `go`), enable Issues + Discussions, and turn on
   **Security → Report a vulnerability** (SECURITY.md points to it).
3. **Branch protection** on `main`: require the CI check + 1 review (CODEOWNERS already gates the
   safety-critical packages).
4. **Homebrew tap** (optional): create `dibakshya01/homebrew-tap` + a PAT, uncomment the `brews:`
   block in `.goreleaser.yaml`, and the next release publishes a formula. `go install` already works:
   `go install github.com/dibakshya01/chaotic-shoebill-stork/cmd/shoebill@latest`.
5. **Launch** (your accounts, your voice): draft copy is in `LAUNCH.md` (git-ignored, never
   committed) — a Show HN, a Reddit r/netsec / r/kubernetes post, and a short social post.
6. **GitHub Marketplace does NOT apply** — it lists only Actions/Apps; this is a CLI/platform.
   Distribution is `go install` + GHCR container + Homebrew + K8s.

## Next engineering milestone (see `ROADMAP.md`)

Complete the Kubernetes operator (real client-go detonators + the autonomous agent), then cloud
attack coverage via Stratus Red Team, then an external evidence witness/transparency log.
