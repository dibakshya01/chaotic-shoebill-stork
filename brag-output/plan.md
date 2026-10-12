# motion-design-video — plan for `chaotic-shoebill-stork`

Resolved options: tone `default`, duration `20s`, landscape `1920×1080`, `30fps`, music: none available (render silent).

## 1. Inspect

**What it is:** An open-source (Apache-2.0, Go) Security Chaos Engineering platform that
detonates a safe, bounded, reversible adversarial condition against your security controls, watches
whether the control prevented / detected / alerted / paged within its SLO, signs the evidence, and
rolls it into a per-service control-efficacy scorecard.

**Who it's for:** Platform-security and SRE teams who deployed security controls (mesh mTLS,
gateway auth, authz policy, PII tokenization, detection/paging) and have no continuous proof any of
them still work.

**Single best moment:** the `FAIL_OPEN` verdict landing — the control that was *supposed* to deny
and didn't. It's the highest-severity, lowest-observability failure, and this tool is what surfaces it.

**Concrete, sourced details (all from README / docs):**
- The 7-step loop: `HYPOTHESIZE → GATE → ARM → DETONATE → OBSERVE → REVERT → ADJUDICATE`.
- The observation ladder: `PREVENTED? DETECTED? ALERTED? PAGED?`
- Verdicts: `PASS | FAIL_UNDETECTED | FAIL_OPEN | FAIL_SLOW | ABORTED | INCONCLUSIVE`.
- Real demo scorecard: `Efficacy 53% · Fail-open 3 open · Coverage 17/21 · Freshness 100%`,
  with Tier-1 `checkout` and `payments` capped at `0.40`.
- Trust claims: "One door to injection" (the Safety Governor); `FAIL_OPEN` dominates;
  `INCONCLUSIVE` never becomes `PASS`; Ed25519-signed evidence (`shoebill evidence verify`).
- Palette (from docs): bg `#0a0e14`, panel `#111722`, line `#222c3d`, ink `#e6edf3`,
  muted `#9aa7b8`, accent teal `#3fd0c9`, pass `#3fb950`, slow `#d29922`, fail-open `#f85149`.

## 2. Beats — angle: "the problem → the proof" (spine: you can't prove your controls hold… until you detonate)

```
[0.0–2.6]   HOOK    text: "Your security controls worked last quarter." → "Do they still?"
                    motion: line 1 mask-wipe in; line 2 slams in bold teal, caret blink
[2.6–6.2]   SETUP   text: "You find out two ways" + two cards: "3 a.m. incident" / "annual audit —
                    samples config, not runtime"  · motion: cards stagger up, ease-out
[6.2–9.6]   TURN    text: "Or you prove it." + 7 loop chips HYPOTHESIZE…ADJUDICATE
                    motion: chips stagger L→R with a connecting line drawing through them
[9.6–13.6]  PAYOFF  the ladder: PREVENTED ✓ · DETECTED ✓ · ALERTED ✓ · PAGED … then verdict stamp
                    "FAIL_OPEN" slams in red with a 4% scale punch · ladder ticks stagger
[13.6–17.4] PROOF   scorecard panel builds: header "Efficacy 53% · Fail-open 3 · Coverage 17/21"
                    + 3 service rows with glyphs (checkout/payments/catalog) · rows stagger in
[17.4–20.0] LAND    🪶 shoebill · "Security Chaos Engineering — with signed evidence."
                    · "Apache-2.0 · github.com/dibakshya01/chaotic-shoebill-stork" · teal underline wipe
```

Running total: 2.6 + 3.6 + 3.4 + 4.0 + 3.8 + 2.6 = **20.0s** ✓
