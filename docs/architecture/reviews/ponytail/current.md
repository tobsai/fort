What this repo does: Fort is a single-owner native AI messenger with a local Node
core and optional runtime adapters. This change adds explicit owner resume while
keeping stopped work paused across reopen. The reviewed load is one owner.

## Semantic review

Reviewer: Codex, October 18, 2026. Reviewed base:
`3cc5fcfce2532b8328bd1c16bef1266d3db076af`. This is a self-review. The immutable
[original Ponytail skill](https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md)
and its recorded SHA256 remain unchanged. The whole-tree baseline is preserved
in [baseline-20261018.md](baseline-20261018.md).

I reviewed every changed source/test/spec path, native model and UI callers,
owner authentication and persisted control state, restart/disposal concurrency,
executor process absence, private host composition, and the expanded native
journey. Stop no longer closes the control listener; it still disposes runtime
work and durably pauses dispatch/scheduling. Full helper exit remains removal's
responsibility. No credential, model, host, permission or billing selection changes.
The review found the expiry race below before final validation. No upstream script
or hook was installed or executed.

## Must fix

1. **Baseline PT1: ordinary resume was absent — fixed in source.**
   - What this is: The owner must explicitly resume stopped work from native Fort.
   - Problem: Stop closed control, while a fresh bootstrap preserved pause without
     an ordinary action that could clear it.
   - Fix: Retain authenticated control/history, reuse the exact runtime-restart
     route, and expose Resume local work. Public and compiled-native tests pass.
   - If we skip it: Stop strands the owner. Installed UI/provider acceptance still
     remains open; source and fixture proof do not waive that gate.

2. **Pairing could expire during resume — fixed.**
   - What this is: Runtime restart may await asynchronous cleanup or initialization.
   - Problem: A request authenticated before that wait could persist running intent
     after its credential expired. A public held-runtime regression returned 200.
   - Fix: Reauthenticate the same request/device inside the transaction immediately
     before running intent. The regression now refuses resume and disposes runtime.
   - If we skip it: Expired authority can resume queued work. The final pinned suite
     includes the regression; no expired real credential was used.

3. **Re-pairing after completed disconnect is still blocked — open.**
   - What this is: Disconnect revokes the old device and retains history.
   - Problem: Durable dispatch pause also blocks the ordinary begin-setup operation,
     including after a fresh core opens. The old credential cannot authorize resume.
   - Fix: Add a separately tested explicit local setup transition after confirmed
     disconnect, preserving pending-recovery and exact-identity checks.
   - If we skip it: The owner cannot pair again normally. This implementation task
     owns the next slice; no v1 or release exception is granted.

## Should fix

4. **Setup timeout fixture assumed an unrealistically short ordering — fixed.**
   - What this is: A test verifies exact retry after setup commits but its reply is lost.
   - Problem: Under the full native suite, the 10 ms timeout could precede the commit,
     so the test no longer exercised its intended committed state.
   - Fix: Hold the fixture response, observe the durable commit before the bounded
     deadline, then release it before exact retry. Production deadlines stay intact.
   - If we skip it: Parallel validation gives intermittent failures without proving
     committed-response recovery. Focused and full native regressions now pass.

## Tests and limits

- Final Node 26.5.0 portable suite: 1,097 passed, zero failed, five optional budgets skipped.
- Native suite: 235 functions passed. Affected Mac process checks: 27 passed,
  three optional budgets skipped. Timing and signed checks were separated.
- Fresh 87-file resources passed every payload hash/size check. The complete
  synthetic native stop/read/resume/reopen/send/revoke journey passed in
  1,496.871292 ms against 5,000 ms, with all owned process groups absent.
- Strict TypeScript and fresh unsigned Mac/iPhone Simulator builds passed.
- Full spec and accepted implementation gates passed. Exact staged coverage is
  checked before commit; a metadata pass does not establish this semantic review.
- [Evidence](../../../../successor/evidence/2026-10-17-node-foundation/owner-resume.md)
  records logs, artifact hashes, the expiry red test, stale-state assertions,
  fixture-ordering and journey-test corrections, and refused Node-version attempts.

Verdict: The explicit resume source slice passes its behavior/build checks.
Re-pairing, deletion scope, standalone packaging and external delivery gates still
block v1. No additional defect was found in the final changed resume paths.

Not checked: Real startup/Keychain, actual Codex/provider/account use, signing or
provisioning, installed UI accessibility/screenshots, physical iPhone, TestFlight,
remote CI or branch protection. Historical/quarantined source was not re-audited.
The full signed sandbox suite was not rerun; this slice does not change its policy.

## Snapshot evidence

The JSON binds the reviewed Git contents and modes. It does not replace the
semantic review or establish permission for any external action.

```fort-review
{
  "format": 1,
  "reviewedCommit": "3cc5fcfce2532b8328bd1c16bef1266d3db076af",
  "reviewer": "Codex",
  "scope": "Explicit owner resume: all 16 changed content paths, native callers, durable auth/lifecycle and executor boundaries; current report excluded from its own digest.",
  "skill": {
    "url": "https://github.com/DietrichGebert/ponytail/blob/2a1fe842d078cf66ec7f3e6096a8ca4279c2cad7/skills/ponytail-audit/SKILL.md",
    "sha256": "2b3223a02c386d1263e1b08343cb8eb3a53e444cdafb1d79e26c8ec2a8fcad28"
  },
  "findings": [
    {
      "id": "PT1",
      "disposition": "fixed",
      "detail": "Ordinary native resume has source, public test, compiled journey and unsigned build proof; installed acceptance remains open."
    },
    {
      "id": "RESUME-EXPIRY",
      "disposition": "fixed",
      "detail": "Observed expiry-during-resume red then green; same credential is checked again before committing running intent."
    },
    {
      "id": "REPAIR-AFTER-DISCONNECT",
      "disposition": "open",
      "detail": "Persisted disconnect blocks ordinary new setup. This task owns a separate explicit local transition; no v1 exception."
    },
    {
      "id": "SETUP-FIXTURE-ORDER",
      "disposition": "fixed",
      "detail": "Held fixture response proves commit-before-timeout and exact retry; production deadlines unchanged."
    }
  ],
  "tests": [
    "1097 portable tests passed on Node 26.5.0; zero failed; 5 optional budgets skipped.",
    "235 native functions and 27 affected Mac process checks passed; 3 Mac optional budgets skipped.",
    "Strict TypeScript and fresh unsigned Mac/iPhone Simulator builds passed.",
    "87 resource payloads verified; final synthetic compiled-native journey 1496.871292 ms, owned groups absent.",
    "Full and accepted implementation spec gates passed; final staged gates required before commit."
  ],
  "exceptions": [
    "Re-pair after disconnect and exact history deletion scope remain open v1 work, not release exceptions.",
    "Real provider/account, startup/Keychain, signing, installed UI, physical device and TestFlight remain unverified.",
    "No remote CI or signed sandbox rerun; historical/quarantined source not re-audited."
  ],
  "files": {
    "specs/059-modular-node-v1.md": "b31be780406639025e05e2610411d4cfeac5d2b950864f02a8f56a1ed7a4285c",
    "successor/V1-ACCEPTANCE.md": "d5c1a7ec29ffca8cc634461ff7efcbf081470bf4aaa9cfa13df35bc6c6564cdb",
    "successor/apple/FortKit/Sources/FortKit/MessengerModel.swift": "cdb8d95353409a6b518c37bc71e6eedf858897add9a4c3247e82ee579a778cc1",
    "successor/apple/FortKit/Sources/FortKit/MessengerView.swift": "dc654da92866bc5d00ae2828fab6c46fe90d66afb8f0eadfb826fa49727b25d1",
    "successor/apple/FortKit/Sources/FortKit/PairingSettings.swift": "0060f9489f1f213348177a79689d0324b28c3923e9cc079224b791e6b16e5aae",
    "successor/apple/FortKit/Tests/FortKitTests/CoreSetupTests.swift": "5c65797bedac0d4634144b95beb2a48b04b34c7bb1380b370f49869e8770065d",
    "successor/apple/FortKit/Tests/FortKitTests/MacHostJourneyTests.swift": "eabf477a4258dee945dd893e0ad06ba352dc812d20711eba14090c19b996e17b",
    "successor/apple/FortKit/Tests/FortKitTests/OwnerLifecycleTests.swift": "76e8c89508954bdabbf41cd651328804ae0cfabcfe04c92dd887e35e99649e95",
    "successor/evidence/2026-10-17-node-foundation/owner-resume.md": "4c1fdd7b6adc3a3e8bdf4552fd1454e340d0d060e7c0adeedcf59cf065294b41",
    "successor/host/fixtures/owner-core.mjs": "cd60cc5a8296c470f4e91b7cd9bb3c4d7bc769a5c91c60771628e22d7eb107fc",
    "successor/host/owned-core.test.ts": "59c330c4489a46ff83ba1431f9d214a4de8660acc4ae4b41919ed9701e4ca2ed",
    "successor/runner/executor-process.test.ts": "542bb4897f55e04aaa33fbcc2891f60de48776bce0202794cf052112e927007c",
    "successor/runner/executor-process.ts": "c04e5a36fc209fb5f17fd1d71764fc52431750e994d649c65ea7fe36d89dcb06",
    "successor/service/openai.ts": "77ed48758a79bb8d8ac2d5a5bc6d08fb98ab78271d84687297d8c87588d84ad6",
    "successor/service/owner-recovery.test.ts": "e58c26b866280af8ad0fad5b1ad12c80989d8e410a42dd5f95081187db08e440",
    "successor/service/owner-resume.test.ts": "629dd6c482a17475a1d8faa1fccab1ee5dceee1a0ce636b3d5b71e2b76684fcb"
  }
}
```
