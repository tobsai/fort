What this repo does: Fort is a single-owner native messenger. This documentation-only
review records the signed account-selection update and its disconnected installed
checks. The substantive source review remains in commit 35882ab7.

Read the exact candidate/install/final reports, the cropped setup screenshot,
accessibility output and closed-ledger absence proof. Verified the distinctions
between the earlier two live selected-model turns, the new synthetic account-reuse
regressions, and the new disconnected installation. No provider reauthentication
or serving-model attestation is inferred from these checks.

Healthy for this bounded update. No unresolved in-scope source finding remains.
The existing duplicate removal-status sentence is cosmetic and recorded as an
exception. App/history remain; all startup, pairing, provider credentials and Fort
processes are absent. The retained ledger is byte-identical across this update.

Verdict: Requested source repair, commit, signed installation and disconnected
checks are complete. Prompt use remains 2/3; no extra provider action was taken.
Not checked: new-source real-provider reauthentication, effective serving model,
physical iPhone, TestFlight/G2 and remote grants. This is not full v1 release signoff.

```fort-review
{
  "format": 1,
  "reviewedCommit": "35882ab7d0aa1c969550c507080b1fbd3440ffef",
  "reviewer": "Codex primary implementation agent",
  "scope": "Documentation-only final signed/disconnected installation evidence; source semantic review is preserved in 35882ab7",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Source 35882ab7: portable1417 pass/16 skips, native345 pass/16 skips, strict typecheck and Release Mac build.",
    "Candidate and installed: 208 files/modes/hashes, 30 binary signatures/certificates, unchanged entitlements/profiles/configuration, native bundle inspector pass.",
    "Exact installed disconnected launch, accessibility inspection, cropped screenshot and normal Command-Q from setup pass. All 208 files still match afterward.",
    "Final absence: no Fort process/startup job/provider auth/client connection; zero devices/enrollment/outbox. Complete ledger hash unchanged and original Messages/runs preserved.",
    "Earlier installed migration build completed two selected gpt-6.1-sol turns, including relaunch; effective model unreported. Budget remains2/3; no further provider access."
  ],
  "exceptions": [
    "Primary-agent evidence review; source review retained in preceding commit, not independently audited.",
    "New-source account reuse is synthetic/compiled-host proof, not a new provider reauthentication after cleanup.",
    "Existing duplicate removal-status sentence is cosmetic and outside this bounded update.",
    "TestFlight/G2, physical iPhone, effective serving-model attestation and remote provider closure remain open."
  ],
  "findings": [],
  "files": {
    "successor/APPLE-HOST-ACTIVATION.md": "0746999c6c234dba6d8b0e350f84b75fd1349caee562e754d87bf4c62b098102",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "9b00c5af5545ed1639097e635c58785d14565c57a8ea7c3daffe5dd0be31a477",
    "successor/V1-ACCEPTANCE.md": "068866e949f2d508223c85cb96c9e318721216838fa060269faa587131c6e2af",
    "successor/evidence/2026-10-17-node-foundation/runtime-migration-0163.md": "900fe022a3262212c65b9430807157e8d86a246509a009ff4966d4cd135e0110"
  }
}
```
