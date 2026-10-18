What this repo does: Fort is a single-owner native AI messenger with a local Node
core and optional runtime adapters. This slice supplies existing-storage inspection
for native removal. Expected load is one local owner and metadata-only inspection.

## Semantic review

Codex self-review, October 18, 2026, base `2c12461`. Used the pinned
[Ponytail skill](https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md)
and preserved baseline. Reviewed storage preparation, all validation helpers, native
service callers, new inspection behavior and refusal matrices. Traced the exact
stalled test process, its one-second stack sample, fixture ownership/reaping and all
seven observation-test callers. Production MacHostProcess was not changed. Reviewed
final source and staged diff, no-creation behavior, measurements and reported limits.

## Must fix

1. **Full native history removal remains unavailable — open.**
   - What this is: Storage inspection is a prerequisite of the new offline ledger seam.
   - Problem: The app still lacks the verified maintenance launch and durable captured
     confirmation linked to completed removal/credential/process-absence checks.
   - Fix: Compose that next boundary and test exact installed behavior separately.
   - If skipped: The source pieces cannot be presented as working product deletion.

2. **Removal inspection could create missing storage — fixed.**
   - What this is: The existing prepare method creates a missing FortHost directory.
   - Problem: Using it during removal changes storage that should only be inspected.
   - Fix: Add inspectExisting and share the validator; skip mkdir/fsync in that path.
     The failing absence test now passes alongside retained and unsafe-path tests.
   - If skipped: A removal check could recreate absent storage and obscure absence evidence.

## Should fix

3. **An observation-test fixture could wait indefinitely after child exit — fixed.**
   - What this is: ObservedChild used Foundation Process and waitUntilExit in an async test.
   - Problem: The full run stalled there with no live child; the sampled stack and exact
     process inspection document the observed failure without claiming an OS root cause.
   - Fix: Own the synthetic child with POSIX spawn/EOF and bounded waitpid. Preserve all
     production observation tests. Only the exact stalled test PID was stopped.
   - If skipped: Routine verification can hang and never produce a trustworthy result.

Verdict: Source/fixture slice verified; continue native maintenance composition. The
shared validator avoids a second filesystem implementation. No dependency, permission,
registration or production process-lifecycle change was introduced.
Not checked: actual App Group resolver/OS container provisioning, real storage or
credentials, installed UI, provider access, new application artifacts or TestFlight.
The interrupted full run is preserved as failed/incomplete; the final full rerun passes.

```fort-review
{
  "format": 1,
  "reviewedCommit": "2c1246173de396af1ac1ecafe41759eaa9adedf7",
  "reviewer": "Codex",
  "scope": "Read-only existing Mac host-storage inspection with shared filesystem validation; synthetic absent/unsafe/replaced storage; repair of the observed test-only child wait hang; exact verification limits.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "NATIVE-HOST-DELETION",
      "disposition": "open",
      "detail": "Maintenance launch, exact durable confirmation and native removal/credential/absence composition remain."
    },
    {
      "id": "INSPECTION-CREATES-STORAGE",
      "disposition": "fixed",
      "detail": "Read-only existing-storage entry point shares validation but skips mkdir/fsync; absent storage remains absent."
    },
    {
      "id": "OBSERVED-CHILD-WAIT",
      "disposition": "fixed",
      "detail": "Sampled Foundation wait hang in synthetic fixture replaced by owned POSIX spawn/EOF and bounded waitpid; production observation unchanged."
    }
  ],
  "tests": [
    "Focused storage: 10 functions passed, three optional skipped, with 25 runtime-directory parameter cases plus unsafe and revalidation matrices.",
    "Seven process-observation tests pass with repaired synthetic child fixture.",
    "Final full native suite: 251 functions passed, ten optional skipped (261 discovered); synthetic compiled host journey 1694.777125 ms and owned groups absent.",
    "20 isolated warm metadata inspections: p50 0.190250 ms, p95 0.195417 ms, max 0.197917 ms; target p95 <=50 ms.",
    "Initial absence test red; initial full native run stalled and was terminated only after exact process/stack diagnosis. No new application artifacts or real account/history actions."
  ],
  "exceptions": [],
  "files": {
    "specs/059-modular-node-v1.md": "ef09969a7bfcc43fd6f71c89704889596262eb8cf562b0fb4d27c279dff70ab7",
    "successor/V1-ACCEPTANCE.md": "5a85227e163e50802ea84a202c2b655e644ff5f7792c6f33874c12b5096d2cfc",
    "successor/apple/FortKit/Sources/FortKit/MacAgentStorage.swift": "d86a1557f5053ca3b17bb216e229a1c6a9a72297116365ad18643b415f54a62d",
    "successor/apple/FortKit/Tests/FortKitTests/MacAgentStorageTests.swift": "7931d2eb504e0a6b8968aaab0ce4cdf1aaa9b2d208d237f2de0391d8ee8d55ea",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostProcessTests.swift": "8c205f67b7e0262e611b08338a75ecfa78b5384b5be2a66757aafe610f3f4546",
    "successor/evidence/2026-10-17-node-foundation/native-storage-inspection.md": "947f273f065c67ff3a4e629174d5f0e3c973f0dec171c62c290de1145e5a844d"
  }
}
```
