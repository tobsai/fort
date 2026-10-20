What this repo does: Fort is a single-owner native messenger. This documentation
review records the tested single-account and composer source update, signed local
installation and final disconnected checks. The source review is preserved in
commit 310b06a9.

Read the signed package, candidate verification, atomic install and final reports,
actual QA/installed screenshots, accessibility results and closed-ledger proof.
The two earlier real prompts are separate from this increment's synthetic send;
no new provider action or effective-model attestation is inferred. All final
installed files match the signed candidate, permissions remain unchanged, and the
retained ledger remains byte-identical.

Healthy for this bounded update. No unresolved in-scope finding remains.
Verdict: Requested source, tests, signed installation and disconnected verification
are complete. Prompt usage remains 2/3.
Not checked: new-source real-provider behavior, actual IME use, physical iPhone,
TestFlight/G2, effective-model identity and remote provider grants. This is not
full v1 release signoff.

```fort-review
{
  "format": 1,
  "reviewedCommit": "310b06a936aa8bb4bcbb8ea99e84167a4f0e0b6f",
  "reviewer": "Codex primary implementation agent",
  "scope": "Documentation-only signed installation and disconnected verification evidence; substantive source semantic review retained in 310b06a9.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Source 310b06a9: portable1423 pass/16 skips; native350 pass/16 skips; strict TypeScript and final Mac Release/QA builds pass.",
    "Rendered synthetic account/mismatch/advanced and visible multiline keyboard/image journey passes; exact stored synthetic request count is one.",
    "Candidate: 208 files/modes/hashes; 30 signatures/certificates; unchanged entitlements/profiles/runtime pins; native inspector passes.",
    "Installed: exact disconnected launch, accessibility, screenshot inspection and normal Command-Q pass; all 208 files reverified afterward.",
    "Final closed ledger unchanged, original Messages/runs preserved; zero processes/startup/provider credential/client connection/devices/enrollment/outbox."
  ],
  "exceptions": [
    "Primary-agent evidence review; source semantic review retained in source commit, not independent audit.",
    "New-source provider reauthentication/catalog/inference and real human IME remain unrun by scope.",
    "Existing duplicated removal-status text is outside this increment.",
    "TestFlight/G2, physical iPhone, effective serving-model attestation and provider-side grant closure remain open."
  ],
  "findings": [],
  "files": {
    "successor/APPLE-HOST-ACTIVATION.md": "5e5c06ce79d2d2a04375bc862db8a819cec5ea03d9b898ceb24a00d351e56fbb",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "937b3027c855bd6709cb788d9ebbc9b2b47516e1f7818868fe31755d6f10a778",
    "successor/V1-ACCEPTANCE.md": "018350c06ffa4ed39cde75107788267128f93ea560050bd01940c0ffb6a8a7d0",
    "successor/evidence/2026-10-17-node-foundation/single-account-composer-20261020.md": "e2feac619d71cc06d85a3c553654280d0f6fae38c16d5a2b1a47b7ddd4fdcbb6"
  }
}
```
