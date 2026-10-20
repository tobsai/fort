What this repo does: Fort is a single-owner native messenger with a Node host and
optional runtime adapters. This review covers native-test scheduling after timing
failures, while preserving the installed authentication handoff and Settings fix.

Reviewer: Codex primary implementation agent. Applied the pinned Ponytail skill and
baseline to the exact workflow/docs changes. Reviewed the timed model/control/startup
path, synthetic backend, account coordination waits, native host-journey budget,
workflow fixture preparation, local instructions and repeated logs. Whole tracked-tree
search located the timing function and its callers/evidence. No product/test code
changes; no dependencies, runtime permissions or persistent-state changes.

## Fixed

1. **Concurrent tests distort main-actor elapsed-time measurements.**
   - What this is: The removal test awaits a separate fixture actor, then resumes
     on MainActor; other tests can occupy that actor before it resumes.
   - Problem: The older focused set without Settings fails three times at
     84.7–97.2 ms versus 50 ms. Alone or alongside Settings it passes. The same
     older set passes three times sequentially. Broader concurrency also causes
     three account-fixture coordination timeouts and a 12.62-second host journey.
   - Fix: Add --no-parallel to the existing native CI command and document it.
     All test code, sample counts, budgets and assertions stay intact. Explicit
     concurrent operations within an individual test still execute.
   - If we skip it: The CI command can report fixture/timing failures caused by
     unrelated test load, obscuring product defects. A narrower split isolated
     removal timing but left the other observed coordination failures.
   - Disposition: Fixed in validation configuration. Three final focused runs pass
     all 25 functions; full sequential suite passes 337, skips 16 optional functions,
     and has zero failures in 59.571 seconds. Remote CI is still unverified.

## Must complete before v1 signoff

2. **Installed and provider acceptance remain pending.** This source-only
   investigation did not inspect or navigate the installed app, generate codes,
   access credentials, change startup or send provider prompts. Authentication
   confirmation, personal account verification/selection, exact Astra discovery,
   final cleanup and installed verification of the committed Settings fix remain
   open. Zero of six prompts used; last observed host/pairing remain active.

Verdict: The parallel failures are reproduced without the Settings regression and
resolved by isolating native tests from one another. The one-option fix preserves
coverage and thresholds. It does not establish installed or real-provider acceptance.
Not checked: Fresh rebuild completion (Swift stalled before tests), remote CI,
portable suite, optional signed/hardware benchmarks, installed AX/screenshot/Quit,
real provider, TestFlight and physical-device execution. All native checks used the
unchanged previously compiled bundle and explicit synthetic resources. G2 stays held.

```fort-review
{
  "format": 1,
  "reviewedCommit": "1af93191f4f7c712229f3e3a1723a0df3029269f",
  "reviewer": "Codex primary implementation agent",
  "scope": "Native CI scheduling isolation, repeated old/new focused controls and complete synthetic native suite; workflow/readme/spec/evidence only, no product or test implementation changes",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Twenty-one diagnostic control runs: older focused set fails all three repetitions without Settings at 84.7-97.2 ms; benchmark alone and paired Settings pass all repetitions; old focused sequential passes all three below 0.41 ms. Removing only the synchronous setup budget does not cure failure.",
    "Initial split invocation passes three focused 24-plus-1 pairs but broader parallel native run has 332 pass, 16 optional skips and four failing functions: three synthetic account coordination waits and the host-journey timing assertion. No thresholds changed.",
    "Complete sequential unchanged native bundle passes 337 functions, 16 optional skips, zero failures (353 total) in 59.571 seconds. Synthetic host journey 2085.27 ms remains below 5000 ms; owned process groups confirmed absent by fixture.",
    "Final workflow YAML parses; its actual native command passes all 25 focused functions on each of three runs, removal p95 0.663/0.841/0.665 ms versus 50 ms. No tests omitted or made optional.",
    "Full spec, accepted implementation, exact staged spec/review coverage and whitespace checks pass. Native product and test files unchanged.",
    "Fresh broad build stalls in Swift pre-planning before test execution; exact task-created runner stopped and exit confirmed. Subsequent checks reuse pinned unchanged compiled test bundle. No fresh build or remote CI pass claimed."
  ],
  "exceptions": [
    "Scoped implementation-agent semantic review, not independent review or whole-v1 acceptance. Pinned skill/baseline and earlier records preserved.",
    "Native tests are now scheduled sequentially in CI. Explicit intra-test concurrency remains. The observed full local run fits the existing 20-minute job timeout; remote runner timing remains unverified.",
    "All comparison runs share normal uncontrolled desktop load. Results isolate competing test work, not actual installed host performance or provider latency.",
    "Installed sign-in UI and host untouched; no account-status refresh, login-code generation or credential inspection. Authentication confirmation, final cleanup and installed Settings Quit verification remain open; zero of six prompts. G2 remains held."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "Unrelated concurrent native tests inflate MainActor elapsed time and delay account-fixture coordination. Old-focused failures reproduce without Settings; isolated/sequential controls pass. Minimal final fix is one --no-parallel option, preserving every test and budget.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "Authentication confirmation, personal-account verification/selection, Astra discovery, provider tests, final cleanup and changed-build installed Settings Quit remain incomplete. Preserve live UI and six-prompt total.",
      "disposition": "open"
    }
  ],
  "files": {
    ".github/workflows/fort-quality.yml": "ac72ce101a05d5002175c4bb8ae98d844d408c3ec29bf0f576739faf394accc4",
    "specs/059-modular-node-v1.md": "9e376e7de4b56cf36dfd6dee209c41cfdb66193256f86734e5c35e4086b662fa",
    "successor/APPLE-HOST-ACTIVATION.md": "6797dea22cab67e7d260016659bf2d5a4cb80161179095ffd2b8c893f4b04c9e",
    "successor/README.md": "7cb5797c589f744926274b95493b07671df633b6f2d404519f49630ea46ec48f",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "247bf2eb57aae78a562efe0234183c89a5da75523ed415aa47a30c953d99cb0a",
    "successor/V1-ACCEPTANCE.md": "96c7253a8ef0c81ede9fa4878362c2ae9bbe01b5db48e18741b2d8847f0d46e6",
    "successor/evidence/2026-10-17-node-foundation/native-timing-isolation.md": "bf462d796e2ccab8011ec735844710d097c583d9deeb9842b749f505a90f9738"
  }
}
```
