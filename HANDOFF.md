# Handoff — what only you can do

The platform is built, tested, documented, and (if push succeeded) pushed. This is the short,
numbered list of the remaining steps that require **your** GitHub account, credentials, or a human
decision. Each has the exact command or click.

> Local toolchain note: this repo was bootstrapped with Go installed at `~/.local/go-sdk` (it was not
> on the system before). On your machine, use your own Go 1.24+ — nothing depends on that path.

## 1. (If not already pushed) push the repo
```bash
cd "chaotic-shoebill-stork"
git remote add origin https://github.com/dibakshya01/chaotic-shoebill-stork.git   # if no remote
git push -u origin main
```
The confidential source doc (`chaotic-shoebill-stork.md`) is git-ignored and was never committed —
keep it that way.

## 2. Enable GitHub Pages for the landing site
Settings → Pages → **Build and deployment: Deploy from a branch** → Branch **main** → **/docs** →
Save. The site is in `/docs` with a `.nojekyll` file, so it serves as-is. It will be live at
`https://dibakshya01.github.io/chaotic-shoebill-stork/`.

## 3. Cut the first release
```bash
git tag -a v0.1.0 -m "chaotic-shoebill-stork v0.1.0 — MVP"
git push origin v0.1.0
gh release create v0.1.0 --title "v0.1.0 — MVP" --notes-file CHANGELOG.md
```
(Publishing a release is outward-facing — it's yours to run.)

## 4. Distribution — pick your channels (GitHub Marketplace does NOT apply)
GitHub Marketplace lists only **GitHub Actions and Apps**; this is a CLI/platform, so it is not
eligible. The correct distribution paths:
- **`go install`** already works:
  `go install github.com/dibakshya01/chaotic-shoebill-stork/cmd/shoebill@latest`
- **Container image** (GHCR): add a `Dockerfile` + a release workflow, or build locally and
  `docker push ghcr.io/dibakshya01/shoebill:v0.1.0`.
- **Homebrew tap**: create `dibakshya01/homebrew-tap` with a `shoebill.rb` formula (use GoReleaser to
  generate it).
- **Krew** (for the operator as a kubectl plugin) — later, once the operator is built.
- Consider **GoReleaser** to automate binaries + checksums + Homebrew + container on each tag.

## 5. Repo hygiene (your auth)
- Add branch protection on `main` (require CI + 1 review; CODEOWNERS already gates the safety-critical
  packages).
- Enable Issues + Discussions; add topics: `security-chaos-engineering`, `chaos-engineering`,
  `security`, `kubernetes`, `go`, `appsec`.
- Turn on the **Security → Report a vulnerability** advisory flow (SECURITY.md points to it).

## 6. Launch (your accounts, your voice)
Draft copy is in `LAUNCH.md` (git-ignored — never committed). It has a Hacker News "Show HN", a
Reddit r/netsec / r/kubernetes post, and a short social post. Post from your accounts when ready.

## 7. Optional next steps
- Record a short demo GIF of `shoebill serve --demo` for the README/site (e.g. with `vhs` or a
  screen recorder) — the three screenshots are already embedded as a stand-in.
- Build the operator in a cluster-dev environment (`operator/README.md`) and run it against a kind
  cluster with real detonators.
- Set up an external evidence witness / transparency log if you want non-repudiation beyond the
  single-key model (documented as out of scope for v1 in `docs/can-and-cannot.md`).

---

### What was done for you
- Full control plane (Safety Governor, verification engine, scoring, Ed25519 evidence ledger), CLI,
  offline demo, REST API, embedded dashboard, 60-experiment catalog, Kubernetes CRDs + controller
  skeleton. All unit-tested offline; `go test ./... -race` green; `shoebill demo` + `evidence verify`
  reproduce the README's claims.
- Research (`finding.md`), spec reviewed to APPROVE over 3 cycles (`build-plan.md`), docs, a landing
  site, CI, and ≥3 adversarial hardening rounds (see `BUILD_LOG.md`).
