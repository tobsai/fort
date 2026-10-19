What this repo does: Fort is a single-owner native messenger with a Node host and
optional official runtimes. This review covers the first approved installed host
attempt and a fresh-state cleanup repair, with no provider sign-in or model request.

Reviewer: Codex primary implementation agent. Reviewed the changed sources and
callers, owner-setup lock/record behavior, MacAgentStorage validation, SMAppService
lifecycle, paired removal, focused tests, artifact delta and all changed evidence.
The whole-tree baseline remains preserved. This is not independent review.

## Fixed

1. **Fresh failed setup left startup registered without a cleanup control.**
   The removal controller previously required a completed pairing. The new explicit
   action holds the existing setup lock, rejects all saved pairing/setup/removal
   records, rechecks bundle identity and exact host-directory absence, unregisters
   through SMAppService and verifies absence again. It deletes no state and cannot
   bypass the existing paired removal flow. Public red/green tests and actual signed
   installed cleanup, normal Quit/reopen and independent absence checks pass.

## Must fix

2. **The helper reads outside its own bundle sandbox during startup.**
   Its program inspector invokes the parent-app inspection path. The canonical
   signed read-only probe gets EPERM opening the parent manifest/startup/profile,
   while its own corresponding files are readable. The approved installed Connect
   failed before storage or pairing. Give helper startup a self-bundle validation
   path and retain full parent validation in the client; keep exact signatures and
   child requirements. Without that repair, registration succeeds but chat cannot
   reach setup. Provider and paired lifecycle acceptance remain unrun.

Verdict: The consumed test's fresh startup registration was removed and verified.
The source cleanup gap is fixed. Helper startup still blocks provider acceptance.

Not checked: Provider sign-in/account/model/controls, messages/cancellation/replay,
paired installed removal, iOS or older-macOS runtime, TestFlight, privacy declaration
acceptance, and broad portable/native suites. No G2 request was uploaded.

```fort-review
{
  "format": 1,
  "reviewedCommit": "092dc52ea62f979f6d5b935371fd0fe95e28c63e",
  "reviewer": "Codex primary implementation agent",
  "scope": "Approved installed test outcome; fresh unpaired startup cleanup controller/storage/UI and tests; exact cleanup artifact and current handoff evidence",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "tests": [
    "Exact original approved report, all 206 files, 30 signatures/public leaf fingerprints, profiles/entitlements and client-side native inspection passed.",
    "Installed Enable passed; one Connect failed before storage/pairing/sign-in. No provider prompts. Scoped job unload alone left startup registration enabled.",
    "Public fresh cleanup test first failed, then passed. Configured focused suite passed 83 functions with six optional timing skips and no failures.",
    "FortMac Release build passed in 31.118 seconds. Signed cleanup candidate changes only client executable, retains exact permissions/runtime bytes, and passes strict nested signing and file checks.",
    "Actual cleanup confirmation reported notRegistered; normal Quit/reopen retained it. Final normal Quit, exact process/job absence, no host/provider credential path and native lock-only metadata passed at 19:29:26 UTC.",
    "Canonical-path read-only sandboxed helper probe reproduces EPERM reading enclosing client files while its own files are readable. No runtime/service/provider launch in probes."
  ],
  "exceptions": [
    "Scoped implementation-agent semantic review, not independent review or v1 signoff. Whole-tree baseline preserved.",
    "Initial restricted fixture run failed three local IPC cases in two existing test functions; configured normal local IPC execution passed. Restricted code-signing trust verification and diagnostic dSYM build needed supported normal system access.",
    "The removal confirmation AX call returned -25205 despite completing the action; observed state was reconciled without repeating removal.",
    "Original 30-minute test ended after cleanup with zero sign-ins and prompts. No grant reset, changed candidate activation, provider cleanup, G2 upload, new identity, visibility change or release."
  ],
  "findings": [
    {
      "id": "1",
      "detail": "An unpaired failed setup had no normal route to unregister startup. Narrow locked absence checks and the explicit cleanup UI now remove only fresh unused registration; installed reopen/absence proof passed.",
      "disposition": "fixed"
    },
    {
      "id": "2",
      "detail": "The helper inspects enclosing client files that its own sandbox cannot read, causing startup failure before storage/pairing. Repair must preserve signed identities without broadening filesystem privileges. Provider, paired lifecycle, TestFlight and physical acceptance remain open.",
      "disposition": "open"
    }
  ],
  "files": {
    "specs/059-modular-node-v1.md": "b4907f62d51c049dea2ff320223664c6933301a4d46ba5bb71ab5476189fc85e",
    "successor/apple/FortKit/Sources/FortKit/MacAgentStorage.swift": "9bc6d9a8f17db7ed8a0d4c0ffcd9c9ace1365ba08889bd87665db6eabfceee56",
    "successor/apple/FortKit/Sources/FortKit/MacHostControl.swift": "6f107a72769a86aa3b0ecac006cce49e0d0ecef077c2396b928c8d6122acc265",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "6e19914037345e59a3a70d81888ff04f1c11447c946418982488ecff928d8ac3",
    "successor/apple/FortKit/Sources/FortKit/OwnerSetupView.swift": "b9667edb2cb40a6d38eec5e8613fa877b73291be2d00d9271f5283a88a30c88a",
    "successor/apple/FortKit/Tests/FortKitTests/MacAgentStorageTests.swift": "23338daa091d1760d48fe6f963852249f2f6add29b5e4a79b603bf6a2ff047dd",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostControlTests.swift": "6a8208f3f407c5c9bd2da7aecd18fb510b5b2ca13dcf5205bc15ccdf2a6271f1",
    "successor/V1-ACCEPTANCE.md": "84b00524429c61d342e4e3ee5f7fe265014baa0abdf27319d5eaa94e4000fac3",
    "successor/APPLE-HOST-ACTIVATION.md": "c6507e6168ab0e6ff253f5405ff7a80d858e1317f2a9a8fac7272613c21eb084",
    "successor/TESTFLIGHT-DELIVERY-20261017.md": "5295d3959f1cd1753e4a316b1f43a7014fa05d688f5168e6bd43ec7432610b39",
    "successor/evidence/2026-10-17-node-foundation/installed-host-proposal.md": "f94a5d68ebb96333ca3e02f6196eff597877fc6abfe56cea00135d20aa503921",
    "successor/evidence/2026-10-17-node-foundation/installed-host-test.md": "5aedf9b5f7784535fbe9ea55c4a4ca8ce27ce2fc244c49a4ec2f8b1611f80324",
    "successor/evidence/2026-10-17-node-foundation/installed-host-test-cleanup.png": "f9dc4694e89821350b1aa9a701c31f1aea4b97eadf69b423e73dac5a24ac858b"
  }
}
```
