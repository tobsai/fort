What this repo does: Fort is a single-owner native AI messenger with a local Node
core and optional runtime adapters. This slice adds explicit deletion of one Mac
client's saved copy and records concrete distribution requirements. Expected load
is one owner, with bounded local caches and no shared hosting.

## Semantic review

Codex self-review, October 18, 2026, base `ddb8917ae6a65d4aa5ab271744d0d9c5fdc3cc3a`.
Read the immutable [Ponytail skill](https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md),
the preserved baseline, changed source/tests/spec/evidence, storage formats and
pairing/recovery callers. Traced descriptor ownership, parsing, identity, bounded
reads, preflight, unlink/sync, failure/retry, model clearing and confirmation capture.
Reviewed packaging hooks/source before the safe ad-hoc check and read-only retained
signature/attribution audit. No external reviewer or independent attestation.

## Must fix

1. **No usable saved-copy deletion — fixed for the Mac client scope.**
   - What this is: The native removal flow retained every local history file.
   - Problem: The owner had no ordinary way to clear a disconnected Mac's saved copy.
   - Fix: A separate confirmation captures the exact installation; public storage
     verifies revocation and credential absence before bounded descriptor-relative
     deletion. It preserves other installations, host data and recovery records.
   - If skipped: The owner must manipulate files manually. Full host-ledger and iPhone
     deletion are still open implementation work, not a release exception.

2. **Changed files could pass an earlier preflight — fixed.**
   - What this is: Pairing credential verification may occur after filesystem reads.
   - Problem: A selected file changing during that check could invalidate preflight.
   - Fix: Verify all selected identities/content after that check, then each file
     before unlink. Public mutation/refusal tests and interrupted retry pass.
   - If skipped: A changed file might be removed using stale assumptions. This still
     does not claim atomic exclusion of hostile code running as the same user.

3. **Confirmation and reopen could misstate deletion — fixed.**
   - What this is: A UI confirmation waits while observable state can change.
   - Problem: Reading the current target only on confirmation could delete another
     installation; reopening also claimed saved history existed after deletion.
   - Fix: Capture the exact connection, reject stale identity, clear projections only
     on success, and derive reopen wording from stored navigation. Model regressions pass.
   - If skipped: The destructive scope or resulting status can differ from what the
     owner saw. Installed accessibility and real Keychain behavior remain unrun.

## Should fix

4. **Codex dependency notices are not a complete shipping inventory — open.**
   - What this is: The current engineering copy preserves root Apache LICENSE/NOTICE.
   - Problem: Its pinned workspace has statically linked dependencies; root files do
     not establish complete third-party attribution for the exact executable.
   - Fix: Finish that source/binary inventory and bind the required notices to final
     package assembly. Apache redistribution permission itself is already documented.
   - If skipped: A package could omit required notices. Owner: implementation agent;
     this is package work, not a request for generic legal approval.

Verdict: The bounded Mac client deletion slice is fixture/build verified. Continue
host-history deletion and package assembly; v1 and release remain unaccepted.
Not checked: real owner data/Keychain, provider or startup activation, account records,
full Codex static dependency closure, installed accessibility/screenshots, physical
Mac/iPhone, TestFlight, remote CI/branch protection. Donor/history baseline preserved.
The unchanged TypeScript portable suite was not repeated; prior 1104 passes are
separate evidence. No dependency or hook was added. No unused-code saving is claimed.

```fort-review
{
  "format": 1,
  "reviewedCommit": "ddb8917ae6a65d4aa5ab271744d0d9c5fdc3cc3a",
  "reviewer": "Codex",
  "scope": "Bounded Mac saved-history deletion, exact native confirmation and distribution requirements audit",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "MAC-SAVED-HISTORY",
      "disposition": "fixed",
      "detail": "Exact scoped client-cache deletion passes synthetic storage/model tests."
    },
    {
      "id": "DELETION-STALE-PREFLIGHT",
      "disposition": "fixed",
      "detail": "Post-credential and per-file validation; changed-root/file and partial retry tested."
    },
    {
      "id": "DELETION-UI-IDENTITY",
      "disposition": "fixed",
      "detail": "Captured connection and honest reopen state; native regressions pass."
    },
    {
      "id": "HOST-AND-IOS-HISTORY",
      "disposition": "open",
      "detail": "Full host-ledger and iPhone cached-history deletion remain implementation work; no release exception."
    },
    {
      "id": "CODEX-NOTICE-CLOSURE",
      "disposition": "open",
      "detail": "Exact statically linked third-party notices must be completed before shipping."
    }
  ],
  "tests": [
    "240 native functions passed at final production source; four focused functions passed with 15 refusal/three mutation cases",
    "20 synthetic 100-snapshot samples p95 43.269041 ms; excludes real Keychain",
    "Final unsigned Mac/iPhone Simulator builds and seven-file mode/content manifests; installed UI unrun",
    "Strict TypeScript passed; unchanged portable suite not rerun (prior 1104 passes)",
    "26 retained Node image hashes/signatures/load commands and 39 notice hashes passed; no runtime invocation",
    "Fresh ad-hoc Node SQLite launch failed at known Team-ID library validation; no validation relaxation",
    "Real history, credentials, providers, startup, account/provisioning and TestFlight unrun"
  ],
  "exceptions": [],
  "files": {
    "specs/059-modular-node-v1.md": "60d73fba9d65f18cfd7bb753d73a28edb3ba8e4d30d58ff6d19ec4e1c57be476",
    "successor/V1-ACCEPTANCE.md": "361945002c8922e9172cac922322d5502485d07747765f844470c41351e4e8ea",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "b9e341b89e1a41e59e9072161db8d05d20bf5d0b5363a38bd9546f707931e7b1",
    "successor/apple/FortKit/Sources/FortKit/PairingSettings.swift": "a978934c64537ea691f2201b61c558fa052d1de8202b4db67ced6821ff5a91f1",
    "successor/apple/FortKit/Sources/FortKit/SavedHistoryRemoval.swift": "a7388c8310a59c52bd6b60961b2651f74807dc398b762382e6dd1889473aa77b",
    "successor/apple/FortKit/Tests/FortKitTests/SavedHistoryRemovalTests.swift": "fd047da3ca2c5c3116c60898f9636b48aab69f41f7bbf7e74d86b5cb010cedfb",
    "successor/evidence/2026-10-17-node-foundation/distribution-requirements.md": "d64f8b913aa07afd55f251ba7405f3619e376d5dd56cbc2208e37b24a7cd5f21",
    "successor/evidence/2026-10-17-node-foundation/saved-history-deletion.md": "bdfa31814a613d9302b24beec23196b7a3cab198082e473e81d9415664b07e25"
  }
}
```
