What this repo does: Fort is a single-owner native messenger with an indexed local
ledger and replaceable runtime. This slice composes separately confirmed retained
host-history deletion. Review assumes one owner and bounded private maintenance,
not shared hosting.

## Semantic review

Codex self-review, October 18, 2026, base cb53e4c. Read the pinned Ponytail method
and preserved baseline. Traced confirmation capture, model gating, production
composition, metadata lease/journal, credential absence, bundle/startup/helper
checks, fixed worker launch/close and cache cleanup through their callers/tests.
Reviewed the shared saved-copy traversal and underlying ledger identity/epoch/
paused-schedule guards. Read both final new files and all changed source/test paths,
compared the final diff, checked legacy call sites and CI fixture inputs, and verified
all files in the fresh inert application. No dependencies were added. No remote CI,
installed product or actual credential/provider behavior was substituted by fixtures.

## Must fix

1. **Credential inspection could hide changed deletion state — fixed.**
   - What this is: The coordinator checks private removal records before maintenance.
   - Problem: A changed journal during credential inspection originally reached deletion.
   - Fix: Recheck cleanup metadata and both journals after credential inspection;
     the observed failing regression now refuses before maintenance.
   - If skipped: An operation could proceed after its locally confirmed state changed.

2. **Maintenance entry could be missing or unsafe — fixed.**
   - What this is: Native inspection and assembly select a fixed offline entry.
   - Problem: Seven unsafe native cases passed, and a manifest-consistent omission assembled.
   - Fix: Apply the bounded native file checks and require the entry in assembly before output.
   - If skipped: A seemingly complete bundle could fail or select an unsafe removal entry.

3. **Compiled maintenance was skipped by existing CI inputs — fixed locally.**
   - What this is: The native job already produces the complete host-resource fixture.
   - Problem: A duplicate environment variable silently disabled the deletion journey.
   - Fix: Reuse FORT_TEST_HOST_RESOURCES; the observed skip is replaced by execution in
     the final full local suite using existing job inputs.
   - If skipped: Native CI could pass without running the compiled deletion journey.

4. **Installed and reachable-iPhone acceptance is still absent — open.**
   - What this is: Local tests and unsigned artifacts now compose deletion.
   - Problem: Secure reachable iPhone host transport remains engineering work; Apple
     account/provisioning, real startup/Keychain/provider and TestFlight are unverified.
   - Fix: Continue the transport work within local preparation and verify exact external
     prerequisites before separately authorized activation/installed tests.
   - If skipped: Source/build completion could be mistaken for working delivered v1.

Verdict: Native deletion composition and its local verification pass; installed
acceptance and reachable transport remain open. Normal removal preserves history.
Presentation reset is shared with device-cache deletion, and the unused duplicate
fixture variable was removed. The legacy generic deletion API remains deliberately
unsupported in production; its tests still protect that refusal.
Not checked: actual App Group ACL/profile compatibility, Keychain/OS latency, real
registration, installed accessibility/screenshots, provider accounts, physical iPhone,
TestFlight, remote CI or branch protection. Initial deletion needs a known epoch;
a separately cleared cache without a prior journal requires a fresh connection.
A saved journal still supports exact retry after cache erasure. No epoch is guessed.

```fort-review
{
  "format": 1,
  "reviewedCommit": "cb53e4c2cc19ed9d0afafa213f85d781ee8d5932",
  "reviewer": "Codex",
  "scope": "Native exact retained host-history confirmation, durable lifecycle/credential/journal/worker/cache composition, required packaged entry and compiled-fixture selection; source, synthetic tests and inert unsigned artifacts.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "CHANGED-DELETION-JOURNAL",
      "disposition": "fixed",
      "detail": "Observed mutation during credential inspection now refuses before maintenance through renewed cleanup and journal comparisons."
    },
    {
      "id": "MAINTENANCE-ENTRY",
      "disposition": "fixed",
      "detail": "Seven unsafe native entry cases and manifest-consistent assembler omission now refuse before use/output."
    },
    {
      "id": "MAINTENANCE-CI-SELECTION",
      "disposition": "fixed",
      "detail": "Compiled journey reuses existing host-resource environment; prior skip reproduced, final full local run executes it. Remote CI remains unrun."
    },
    {
      "id": "INSTALLED-AND-IPHONE-ACCEPTANCE",
      "disposition": "open",
      "detail": "Reachable secure iPhone host transport remains implementation work; Apple account/profiles, installed lifecycle/UI/provider and TestFlight availability remain unverified."
    }
  ],
  "tests": [
    "Coordinator/model behavioral failures and journal mutation failure preserved; fixed controls cover fifteen barriers, five interrupted/concurrent cases and three worker cases.",
    "Final full native: 271 discovered, 261 passed, ten optional skips, zero failures, 52.038 seconds; compiled maintenance executes with existing CI inputs.",
    "Twenty isolated synthetic coordinator retries: p50 21.054 ms, p95 23.066 ms, maximum 23.189 ms against 500 ms target.",
    "Strict TypeScript and 21 assembly tests passed with zero assembly skips.",
    "Full portable: 1158 passed, six optional skips, zero failures; initial restricted fixture failures/stall preserved and owned run stopped before permitted rerun.",
    "Fresh unsigned Mac/helper/iPhone Simulator builds passed; all 186 review-app file hashes and sizes verified, 414957609 bytes."
  ],
  "exceptions": [
    "Only disposable synthetic data deleted; no provider/account/credential/startup/network activation, signing, installation or distribution.",
    "Initial exact confirmation requires a known saved epoch; after independent cache erasure without a journal, reconnect to establish scope. Pending journal retry does not require retained cache.",
    "Whole package identity currently includes pinned optional Codex metadata even though offline deletion never invokes it. Core/ledger and maintenance operation remain provider-free.",
    "CoreSimulator version mismatch prevents iOS execution; compile/test-bundle evidence does not establish physical acceptance."
  ],
  "files": {
    "specs/059-modular-node-v1.md": "565a7370096390c209c341901799d7891d36a483595ee5890a162ac12ac2f0ab",
    "successor/V1-ACCEPTANCE.md": "528abb323b612449aecbeb8152609af1c2cd585ace7a2cb59e78058355720389",
    "successor/apple/FortKit/Sources/FortKit/MacHistoryMaintenance.swift": "886bc1c80b5c94a96c6e08a72b2385aa3141b54624788ce4efd4ed5094d3e735",
    "successor/apple/FortKit/Sources/FortKit/MacHostControl.swift": "f47650c042840aacfe4075976ba7538d7cbcad58d9607c0a4ff9c638eeab2e14",
    "successor/apple/FortKit/Sources/FortKit/MacHostProgram.swift": "0eb6fd1fe1334f75204c5999745894a3cd52cfedb01199da54f0c0aaef7d5a4f",
    "successor/apple/FortKit/Sources/FortKit/MacHostRemoval.swift": "1cd74dfb4a68e8a4ea212bc203033145b673dd0c2d38ab75bf6c2aee5e1e5926",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "ed8d09a740fd92a1216e0b30332cf1ed33cceddf914081812a95abf50f41daa7",
    "successor/apple/FortKit/Sources/FortKit/OwnerSetupFiles.swift": "35e7ececfcd6a53568346808f43834f83abc94164b8b9ce6c20894f243dc2605",
    "successor/apple/FortKit/Sources/FortKit/PairingSettings.swift": "e0d91930fe06af766a6fc0117bcee24c9982e0f00073bbd341d41497973bf7b0",
    "successor/apple/FortKit/Tests/FortKitTests/MacHistoryDeletionTests.swift": "cb968dfcf3e066f5e5a3bdfbc44b707abbe64a2bab5a2e81a0dfadd39b71511a",
    "successor/apple/FortKit/Tests/FortKitTests/MacHistoryMaintenanceTests.swift": "9aa2997ed6f476391f2c6d92295b306d87834488afd7e87346531ae3a846737e",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostProgramTests.swift": "51352385150d223df6c872288e004eda1ed3af406aa69f92cd11ddc3cdc1ec63",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostRemovalTests.swift": "aa1f981e58779d65e1ad5e8944192029df20527ecd0af82fc828a6a2e72bfd41",
    "successor/evidence/2026-10-17-node-foundation/native-history-removal.md": "5cc18922f1c3e1aea4dab8f4572b5f6499f2888a3641a80331ccec0eb3c2a8ac",
    "successor/packaging/mac-application.test.ts": "f44437208081fe0bfa388c36ef945a27109094dd2aaf64f37eb0faf106ff06a7",
    "successor/packaging/mac-application.ts": "6cf9dc375e148968f9c2fe661f89286b59fc9273af27c6681dc2e861448e4e40"
  }
}
```
