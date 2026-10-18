What this repo does: Fort is a single-owner native AI messenger with a local Node
core and optional runtime adapters. This slice permits explicit new local pairing
after confirmed disconnect while preserving history and old-token revocation.

## Semantic review

Reviewer: Codex, October 18, 2026. Reviewed base:
`b2b005c2b7fd896c434fe06e8e98a00bb3a22f15`. This is a self-review using the pinned
[original Ponytail skill](https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md).
The immutable skill hash and whole-tree baseline are unchanged.

I reviewed all changed source, tests and spec paths, every setup-completion caller,
the private pipe's awaited callbacks, native setup eligibility, owner credentials
and offers, persisted stop state, runtime preparation and close coordination. Load
is one owner. New setup stays on the existing trusted local contract. Begin creates
only an offer; completion validates before runtime work and again before the atomic
pairing/running commit. Existing authenticated resume uses the same bounded runtime
operation, retaining its post-await credential check.

## Must fix

1. **Re-pairing after disconnect was blocked — fixed in source.**
   - What this is: Disconnect revokes the old device while retaining history.
   - Problem: Persisted pause prevented the ordinary setup flow from starting again,
     even in a fresh core. Both public red cases reproduced hostUnavailable.
   - Fix: Permit an explicit new offer only after completed revocation and supported
     runtime cleanup; prepare runtime on exact completion, then commit the new pairing.
   - If we skip it: The owner cannot reconnect normally. Public, private-channel and
     compiled-native checks now pass; installed Keychain/provider acceptance remains open.

2. **Completed uninstall still blocks a later setup — open.**
   - What this is: Native setup refuses any saved host-removal record.
   - Problem: The confirmed record survives cleanup. A later installation with retained
     owner files has no explicit reconciliation action that can make setup available.
   - Fix: Inspect and implement a separate exact-identity, confirmed-removal transition;
     preserve history and pending/unconfirmed recovery protection.
   - If we skip it: Reinstall with retained owner data can remain blocked. This task
     owns the remaining local lifecycle work; no release exception is granted.

No additional defect was found in the final changed pairing paths. Wrong identities,
old devices, canceled or expired offers, unconfirmed cleanup, unavailable runtime,
overlap and close during preparation are refused. Failed preparation leaves pairing
absent and dispatch paused. Exact successful completion retries reuse their receipt.
The async caller migration includes both fixtures that previously read the receipt
synchronously; no legacy synchronous completion API remains.

## Tests and limits

- 54 focused setup/resume/private-channel functions passed.
- Node 26.5.0 portable suite: 1,104 passed, zero failed, five optional budgets skipped.
- Native suite: 232 functions passed with all budget, retained-signature and complete
  journey functions excluded. The earlier 235 count also included three timing functions.
- Isolated complete compiled-native journey: 1,668.438 ms against 5,000 ms, with all
  owned process groups absent after final recovery/shutdown.
- Strict TypeScript passed; 87 rebuilt resource payload hashes and sizes passed.
- Native product code is unchanged; prior fresh unsigned Mac/iPhone builds remain
  the artifacts. The changed native test and compiled core were rebuilt and executed.
- [Evidence](../../../../successor/evidence/2026-10-17-node-foundation/owner-repair.md)
  records artifact hashes, initial red cases, the corrected async test callback and
  the journey's corrected distinction between ordinary Disconnect and full recovery.
- Full/accepted spec and exact staged review gates are required before commit.
  Content/mode coverage cannot establish the truth of this semantic review.

Verdict: Re-pairing passes the source and fixture checks. Reinstall reconciliation,
history-deletion scope and external package/delivery gates still block v1.

Not checked: Real Keychain/startup, provider/accounts, signing/profile provisioning,
installed accessibility/screenshots, physical iPhone, TestFlight, remote CI or branch
protection. No new signature/sandbox benchmark or unrelated Mac adapter rerun was
needed for unchanged policy/adapter code. Historical/quarantined source was not
re-audited. No external account, credential or persistent grant was used.

## Snapshot evidence

This metadata binds reviewed Git contents and modes, not external authorization.

```fort-review
{
  "format": 1,
  "reviewedCommit": "b2b005c2b7fd896c434fe06e8e98a00bb3a22f15",
  "reviewer": "Codex",
  "scope": "All nine changed content paths for explicit local pairing after disconnect; setup callers, private channel, runtime lifecycle and native eligibility reviewed.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "REPAIR-AFTER-DISCONNECT",
      "disposition": "fixed",
      "detail": "Explicit local setup can complete after confirmed revocation; source/private-channel/compiled-native proof passes. Installed acceptance remains open."
    },
    {
      "id": "REINSTALL-REMOVAL-RECORD",
      "disposition": "open",
      "detail": "Confirmed host-removal records still block new setup; exact retained-history reconciliation remains local implementation work, not a release exception."
    }
  ],
  "tests": [
    "54 focused setup/resume/private-channel checks passed.",
    "1104 portable passed; zero failed; 5 optional budgets skipped.",
    "232 native functions passed with budget, signature and complete journey checks excluded.",
    "Isolated compiled-native journey passed in 1668.438 ms, owned groups absent.",
    "Strict TypeScript and all 87 resource payload hashes/sizes passed; native product source unchanged from prior unsigned builds."
  ],
  "exceptions": [
    "Reinstall reconciliation and history-deletion scope remain open v1 work.",
    "Real provider, Keychain/startup, package/signing/profile, installed UI, physical devices, TestFlight and remote CI unverified.",
    "Historical/quarantined source not re-audited; unchanged signature/sandbox and Mac adapter suites not rerun."
  ],
  "files": {
    "specs/059-modular-node-v1.md": "59eb8448f4d97a690eb4dc791cf8362175399ad7adf637cef1e3ebd02e7839ab",
    "successor/V1-ACCEPTANCE.md": "df77c39e3c28ccd1aa7a9ed0ca9d6acde3c328fff2c1ab9abf93fa7cd0480e2e",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostJourneyTests.swift": "a8e3adfd4549c24b1cf4731c23cb4c43f2be0903b0aab5bdfa811066fd05b3e6",
    "successor/evidence/2026-10-17-node-foundation/owner-repair.md": "8aee40ca460d85d44d4b6bf362fa1b07cedb03ed58e2657c14b42666c90e7c8d",
    "successor/host/fixtures/owner-core.mjs": "a2f37b97703437796b2415397a95b06b3be7c4ba19ea72910d9779f4149c726d",
    "successor/host/fixtures/private-core.mjs": "7abc1f77e9664458bdecd2c2a7d84169b696a3aa7fbe5e41429451515703f5fc",
    "successor/service/openai.ts": "5b455a9062f6c50b9daf1140cd0baf2712cd5473f13940d6d16576c1e79c5fad",
    "successor/service/owner-repair.test.ts": "9568e004282c8065065fbbf0badf37b77d3575a8e28856bd2af2f1159d33fafa",
    "successor/service/owner-setup.test.ts": "08a18eb41777553612adaa9cccf073ab16c03bab81a6654c5d63070582c3a3f9"
  }
}
```
