# Writing experiments

An experiment is **data**: a declarative hypothesis and its assertions. It never contains a payload
with side effects. Files live under `catalog/<family>/<id>.yaml` and are validated by `shoebill
catalog validate`.

## The hypothesis-first rule

Write the hypothesis and its steady-state probe **before** the injection. If you cannot state what
must remain true and measure it right now, you are not ready to detonate. The validator enforces the
mechanical half of this: inject-mode experiments must declare at least one steady-state probe.

## Anatomy

```yaml
apiVersion: chaos.shoebill.dev/v1alpha1
kind: SecurityChaosExperiment
metadata:
  name: mesh-06-sidecar-fail-open      # unique, kebab-case
  version: 1
  title: Mesh authorization under sidecar failure
family: mesh                           # one of the nine families
blastRadiusClass: high                 # low | medium | high
description: >
  A clear statement of the control under test and why it matters.
controls: [service-mesh-authz, mtls]   # generic, vendor-neutral control names
attackIds: [T1562.001, T1078]          # >=1 required
owaspIds: [API5:2023]                  # owaspIds OR controlFamilies required
controlFamilies: [ISO-A.8.5, PCI-DSS-7]
hypothesis:
  statement: >
    When the sidecar cannot evaluate policy, all inbound requests are denied.
  steadyState:                         # required for inject mode
    - source: prometheus
      query: 'sum(rate(mesh_requests_total{service="{{.target}}"}[2m])) > 0'
      expect: {type: threshold, op: gt, value: 0}
injection:
  detonator: k8s                       # mock | http | observe | k8s | istio | dns | cert | network
  mode: inject                         # inject | observe_only
  spec: {action: container-kill, container: sidecar-proxy}
  duration: 45s                        # required, positive, for inject mode
  reversible: true                     # required true for inject mode
  revertVerification:                  # independent check that the undo worked
    source: prometheus
    query: 'up{job="sidecar",service="{{.target}}"}'
    expect: {type: threshold, op: eq, value: 1}
assertions:
  - stage: PREVENTED
    source: gwlog
    query: 'service="{{.target}}" AND code=200 AND NOT has_field("authz_decision")'
    expect: {type: absent}             # a match here means the control FAILED OPEN
    sloMs: 0
    requireUuid: false                 # justified below
    justification: >
      Fail-open is asserted over ALL traffic during the window, not only synthetic
      traffic, so UUID scoping would miss real fail-open behaviour.
    securityRelevant: true             # a matched absent assertion ⇒ FAIL_OPEN
  - stage: DETECTED
    source: prometheus
    query: 'increase(sidecar_unhealthy_total{service="{{.target}}"}[2m])'
    expect: {type: threshold, op: gt, value: 0}
    sloMs: 60000
  - stage: ALERTED
    source: alertmanager
    query: SidecarUnhealthy
    expect: {type: present}
    sloMs: 300000
blastRadius: {maxPods: 1, maxPctReplicas: 5, maxDurationSeconds: 60, maxTrafficPct: 1}
expectedNoise: [PodContainerRestart]   # must NOT include any asserted alert rule
promotion: {requiresPassIn: [playground, stage, preprod], withinDays: 14}
capabilities: [mesh-injection, prod-injection]
```

## The fail-open pattern

The highest-value assertion. A control that *permits* something it should deny is a `FAIL_OPEN`:

```yaml
- stage: PREVENTED
  source: gwlog
  query: '<the forbidden success, e.g. 200 without an authz decision>'
  expect: {type: absent}          # if this MATCHES, the forbidden thing happened
  securityRelevant: true          # ⇒ verdict FAIL_OPEN (dominates everything)
  requireUuid: false              # absence is over all traffic, not just synthetic
  justification: '...'            # required whenever requireUuid is false
```

## Expectation types

| type | meaning |
|------|---------|
| `present` | the query returned a (UUID-correlated, unless justified) result |
| `absent` | the query returned nothing; a match on a `securityRelevant` absent ⇒ FAIL_OPEN |
| `threshold` | numeric compare: `op` ∈ gt/gte/lt/lte/eq/neq against `value` |
| `count` | compare the number of results against `value` |
| `value` | exact value compare (e.g. HTTP status `eq 401`) |
| `paged` | a page was delivered |

## observe_only experiments

Drift detection, policy diffs, and enumeration never inject — they are always safe to run, even in a
freeze window. Use `detonator: observe`, `mode: observe_only`, a zeroed `blastRadius`, and no
`reversible`/`duration`.

## Validate before you commit

```bash
shoebill catalog validate ./catalog
```

The validator checks: standards anchoring (≥1 ATT&CK + ≥1 OWASP/compliance), inject ⇒
reversible + steady state + positive duration + `maxDurationSeconds > 0`, `requireUuid: false` ⇒ a
written justification, `expectedNoise ∩ asserted alert rules = ∅`, and valid enums throughout.
