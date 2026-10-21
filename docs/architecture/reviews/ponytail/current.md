What this repo does: Fort is a single-owner native messenger with a Node core and optional official runtime adapters. This review covers one CI repair slice and assumes one owner with a two-job CI (Ubuntu portable, macOS native).

Reviewed every changed path plus the callers of `codexLaunchPlan`, `createMvpRuntime` and `validateOwnedProcessOptions`, and the suite classifier. No product source changed: only test files, the suite inventory, one npm script and a Swift source list in the signed-suite probe.

**Must fix (resolved)**

1. **Portable job runs macOS-only tests** (`successor/prototype/owned-codex.ts:13`)
   - **What this is:** `codexLaunchPlan` builds the reviewed Codex launch arguments for the pinned macOS runtime.
   - **Problem:** It refuses non-darwin hosts, but 16 portable tests call it, so the Ubuntu job always failed.
   - **Fix:** `owned-codex.test.ts` is classified `macos`. The other cases move unchanged into `*.macos.test.ts` files.
   - **If we skip it:** The portable job stays red and hides real regressions.

2. **Guardian fixture misses a Swift source** (`successor/host/guardian.test.ts:15`)
   - **What this is:** The test compiles a hand-picked list of FortKit files with `swiftc`.
   - **Problem:** `MacCoreProcessTransport.swift` now uses `MacHistoryDeletionRequest`, which lives in `MacHistoryDeletion.swift`, a file that was not in the list. Compilation fails.
   - **Fix:** Add the file to this list and to the same list in `packaging/sandbox-feasibility.ts`.
   - **If we skip it:** The native job fails before running any guardian test.

**Should fix (open)**

3. **Launcher-death timing in the native job** (`successor/runner/codex-executor.test.ts:200`)
   - **What this is:** The test checks that killing the launcher removes every nested process within 3 s.
   - **Problem:** The test missed its deadline once in CI, in the same run where finding 2 failed. The cause is unknown and can't be reproduced without macOS.
   - **Fix:** None yet. Re-check after the native job runs with finding 2 fixed.
   - **If we skip it:** The native job may stay red.

Verdict: findings 1 and 2 are fixed. Finding 3 needs one native CI run.
Not checked: any macOS execution, Swift builds, provider behavior and installed apps.

```fort-review
{
  "format": 1,
  "reviewedCommit": "cffe81a055f9e5b5f739ac197cd0b8d6dca02f77",
  "reviewer": "Claude Code (cloud session) for Toby",
  "scope": "CI repair slice 0f64b37f..cffe81a0: test classification for the macOS-only Codex launch plan, guardian/sandbox Swift source lists, suite inventory test and test:prototype script. Callers of codexLaunchPlan, createMvpRuntime and validateOwnedProcessOptions plus verification/test-suite.mjs were read. No product source changed.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Linux, Node 26.5.0: strict TypeScript passes; portable suite 1423 tests, 1403 pass, 15 skips, 5 fail. The 16 failures from Fort quality run 39527002641 are gone. The 5 remaining failures (owned-process descendant/acknowledgement timing and unreadable-evidence) did not fail in that remote Ubuntu run; they reproduce only in this root-user container and are environment-specific.",
    "Spec gates: test:specs, test:reviews and validate-specs --implementation for Spec 059 pass.",
    "macos suite listing shows the four reclassified files; suite inventory test passes.",
    "Unrun: the macOS suite, swiftc compile of the guardian fixture and FortKit swift test. No macOS host is available here; the PR's native CI job is the first execution."
  ],
  "exceptions": [
    "No macOS execution in this slice; native correctness is proven only by the remote native job."
  ],
  "findings": [
    {
      "id": "portable-runs-darwin-only-plan",
      "detail": "The Ubuntu portable job ran 16 tests that build the pinned macOS Codex launch plan, which throws on non-darwin by design (owned-codex.ts:13). They are moved into macOS-classified files with unchanged bodies.",
      "disposition": "fixed"
    },
    {
      "id": "guardian-missing-history-source",
      "detail": "guardian.test.ts compiled MacCoreProcessTransport.swift without MacHistoryDeletion.swift, which bb2086d made a dependency, so swiftc failed in the native job. sandbox-feasibility.ts had the same list gap and got the same fix.",
      "disposition": "fixed"
    },
    {
      "id": "launcher-death-timing",
      "detail": "runner/codex-executor.test.ts 'abrupt launcher death...' timed out at 3 s in the native job while the guardian test was failing in the same run. It cannot be reproduced here and may be load-related. It is left unchanged pending the next native run.",
      "disposition": "open"
    }
  ],
  "files": {
    "successor/host/guardian.test.ts": "b0602eaad5fc8050db63a2624309c643b05a003c25bde47021f80e0e0bc2c23e",
    "successor/package.json": "5d2f05ded52d32b7edb776403a5146ec2029581ee9ae6d67d2f9e9fad22ec35a",
    "successor/packaging/sandbox-feasibility.ts": "d3539e2b65ccc20f72f0681db2ab2dddd5483f279fbe8625d808e355142f5325",
    "successor/prototype/mvp-session.macos.test.ts": "12575036150fbe2c91dc8de79bdd9ce559970274e412a4d192495a18e6d0e172",
    "successor/prototype/mvp-session.test.ts": "7002986c542653fd712b5eb6b6523696a0d1967335660def6282187847361608",
    "successor/prototype/owner-demo.macos.test.ts": "6491a6742aaf3b9fccb1a010a7baace93b6b14de0db37624f30360f94eb6a115",
    "successor/prototype/owner-demo.test.ts": "04a7103fc0baf0aeff8891f1b1c41fcb4e8aaeb7c72d757a31bee8479e651019",
    "successor/prototype/session-budget.macos.test.ts": "2e1c7884fe42c652be4bac9c6677c5fedce9da29652913912c9d18e5baf97e06",
    "successor/prototype/session.test.ts": "bfdd8138cc705eaadf3acf7389d9a00554c9872c0206af52f67e0fc676acac9e",
    "successor/verification/test-suite.mjs": "2c14e1c20a8a671b34fd0f44132e6f4de2ae3a5b03ffaa171e1bafb2bdd33843",
    "successor/verification/test-suite.test.mjs": "2f1f8fff6c50e90b314b732dbca5cbecdfff905ad0d8d06cefc45d9be23bafed"
  }
}
```
