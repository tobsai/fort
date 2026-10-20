What this repo does: Fort is a single-owner native messenger. This documentation-only
checkpoint records the exact installed migration and pending attended sign-in.

Read the actual signed reports, retained-ledger rehearsal, original/current row
comparison, installation receipt and native control outputs. Prior source semantic
review is preserved in d8d76bc3. All historical rows were preserved; source-version
rollback is no longer safe after subsequent setup writes. No stale backup restore
is authorized by this report.

Must fix before completion: owner sign-in, exact 6.1 selection/inference and final
local startup/pairing/provider-credential cleanup remain pending. Zero of three
prompts used. Do not mistake the offline catalog or paired relaunch for inference.
Record the failed direct AX action and capture limit without claiming they passed.
The inspected image containing dimmed conversation content was removed.

Verdict: Installed migration and paired Quit are verified; wait only for the normal
owner sign-in step before dependent live checks and cleanup.
Not checked: completed authentication, exact 6.1 inference, cleanup absence,
physical iPhone, TestFlight/G2, full OS host isolation or remote provider closure.

```fort-review
{
  "format": 1,
  "reviewedCommit": "d8d76bc3b6268fb83b64db04fb88f67b32a8386b",
  "reviewer": "Codex primary implementation agent",
  "scope": "Installed migration, signed package and native lifecycle evidence checkpoint; owner sign-in pending; documentation only",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Prior source commit: portable1398 pass/16 skips, typecheck, crash/retry/rollback public regressions. No code change in this evidence checkpoint.",
    "208 file hashes/modes, 30 signatures/certificates, identical entitlements, native bundle check passed.",
    "Real closed-ledger copy apply/retry/rollback and exact logical restoration passed. Actual migration and independent historical-row/metadata comparison passed.",
    "Installed setup/pairing, explicit Resume, observed stop dialog Escape Cancel, normal Command-Q and paired relaunch passed. Direct AX Cancel and first window screenshot did not pass.",
    "Normal managed device-code challenge ready and verification page opened; 0/3 prompts used. Owner action, model inference and final cleanup pending."
  ],
  "exceptions": [
    "Primary-agent evidence review, not independent source audit. Prior source semantic review retained in Git.",
    "No retained screenshot claiming transcript-free capture; dimmed conversation image removed.",
    "App/host/pairing temporarily remain active for owner sign-in. Startup/pairing/local-provider cleanup must follow smoke; remote grants remain outside cleanup."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Owner must complete the normal managed device-code sign-in before exact 6.1 inference can be tested. No prompt used; local activation must be cleaned afterward.",
      "disposition": "open"
    }
  ],
  "files": {
    "successor/evidence/2026-10-17-node-foundation/runtime-migration-0163.md": "613c28780ce50e9934b40d89e76c5d9a69ff9f79f16b143f6a8ae5c18d868fbe",
    "successor/V1-ACCEPTANCE.md": "42543be63784f76c7b45f7ccb57bf530b3bfbf2aedb6618c14f18ef319946f59",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "ac96165d76b838720261a91fa4bdbf961d19a74020badfab2e4385a161b004c5",
    "successor/APPLE-HOST-ACTIVATION.md": "54fce2cbc5a32a99a0a76c3b1de258e42d12f079658f0ccc785c97b262080654"
  }
}
```
