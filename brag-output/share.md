# Share copy — shoebill launch video

Tone: default. Every claim below is in the video and sourced from the repo.

## X / Twitter

You deployed world-class security controls. What you don't have is proof any of
them still work.

🪶 shoebill is open-source Security Chaos Engineering: safely detonate an
adversarial condition, watch whether the control PREVENTED / DETECTED / ALERTED /
PAGED, sign the evidence, and score it.

The verdict it exists to catch — FAIL_OPEN — is the one nobody tests.

Apache-2.0 · github.com/dibakshya01/chaotic-shoebill-stork

## Show HN / LinkedIn

**Show HN: shoebill — prove your security controls actually work (Security Chaos Engineering)**

Control efficacy is usually discovered one of two ways: at 3 a.m. during an
incident, or annually during an audit that samples configuration, never runtime
behavior. The worst class — fail-open under stress — is the highest-severity,
lowest-observability failure there is.

shoebill runs one governed loop: HYPOTHESIZE → GATE → ARM → DETONATE → OBSERVE →
REVERT → ADJUDICATE. A Safety Governor is the only door to injection; nothing
detonates without a signed, scoped, time-boxed capability. Every artifact is
correlated on one run UUID, so coincidental alert noise can never score a false
PASS. FAIL_OPEN is its own verdict and dominates; INCONCLUSIVE never becomes PASS.
Results roll into a per-service, Ed25519-signed control-efficacy scorecard an
auditor can verify without trusting us.

Go, Apache-2.0: github.com/dibakshya01/chaotic-shoebill-stork

## Notes

- Rendered silent: no cleared music track was available on this machine. To add
  one, drop an MP3 in and re-run with it as the last arg to `build.sh` (see below).
- Poster frame: `poster.png` (the 7-step loop).
