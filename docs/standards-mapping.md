# Standards mapping

Every experiment is anchored to authoritative standard IDs, so a passing (or failing) run is
audit-usable evidence, not just a log line. Versions verified October 2026.

> A clean automated check means "no violation **detected**", not "control proven present for all
> inputs". Label honestly — see [what it can & cannot do](can-and-cannot.md).

## Frameworks referenced

| Framework | Version / date | Used for |
|-----------|----------------|----------|
| **MITRE ATT&CK** | v19 (v19.0 Apr 2026; v19.2 current). Enterprise + Cloud (IaaS/Identity) + Containers matrices. *Defense Evasion* split into Stealth + Defense Impairment — technique IDs unchanged. | `attackIds` on every experiment |
| **MITRE ATLAS** | current | adversarial-AI techniques for the `ai` family |
| **OWASP API Security Top 10** | 2023 | gateway / mesh authz |
| **OWASP Top 10 (Web)** | 2025 | supply chain (A03), logging/alerting (A09) |
| **OWASP Top 10 for LLM Applications** | 2025 (v2.0) | `ai` family |
| **OWASP Top 10 for Agentic Applications (ASI)** | 2026 | `ai` family |
| **OWASP AISVS** | v1.0/1.01 (2026) | `ai` family (incl. C10 MCP) |
| **PCI-DSS** | v4.0.1 (2024) | auth (7/8), data protection (3.5), logging (10) |
| **ISO/IEC 27001:2022** | 2022 (Annex A, 93 controls) | cross-cutting |
| **NIST SP 800-53** | Rev 5.2.0 (2025) | AC/IA/SC/AU/IR/CM/CP families |
| **NIST CSF** | 2.0 (2024) | PR.AA, PR.DS, DE, RS, RC |
| **SOC 2 (AICPA TSC)** | 2017/2022-PoF | CC6/CC7/CC8 |
| **India DPDP** | Act 2023 + Rules 2025 | PII tokenization / masking (§8(5)) |

## Family → standards (representative)

| Family | MITRE ATT&CK | OWASP | Compliance |
|--------|-------------|-------|------------|
| Service Mesh | T1557 AiTM, T1078 Valid Accounts, T1562.001 Impair Defenses | API5, API8 (2023) | ISO A.8.5/A.8.24, PCI-7, NIST SC-8/AC-3 |
| Gateway & Edge | T1078, T1550.001 App Access Token, T1190 Exploit Public-Facing App, T1498 Network DoS | API1/API2/API5 (2023), A03:2025 | PCI-8.2, SOC2-CC6.1 |
| Identity & Secrets | T1552.001 Creds in Files, T1552.005 Cloud Metadata, T1098, T1553 | API5 (2023) | ISO A.8.2, NIST IA/SC-12, PCI-8.6 |
| Network & Perimeter | T1210, T1557, T1552.005, T1071.004 DNS | A01:2025 | ISO A.8.20/8.21, NIST SC-7, PCI-1 |
| Data Protection | T1530 Data from Cloud Storage, T1565.001 | LLM02:2025 | DPDP-S.8, PCI-DSS-3.5, ISO A.8.11 |
| Supply Chain | T1610 Deploy Container, T1611 Escape to Host, T1195.002 Dependency, T1574 | A08:2025 | ISO A.8.28, PCI-6, NIST CM |
| DR & Resilience | T1078, T1562 Impair Defenses | — | NIST CP, SOC2-A1 |
| Detection & Response | T1562, T1070 | A09:2025 | NIST AU-6, SOC2-CC7.2, ISO A.8.15/8.16 |
| AI / Agentic | T1059/T1204, T1078, T1567 Exfil over Web Service, T1195.002 | LLM01/LLM02:2025, ASI01/ASI02/ASI03/ASI04/ASI06, AISVS C5/C9/C10 | ISO A.8.x, DPDP-S.8 |

To see a specific experiment's mapping:

```bash
shoebill catalog show mesh-06-sidecar-fail-open
```

Every receipt carries `attack_ids`, `owasp_ids`, and `control_families`, so an auditor can filter
evidence by control framework.
