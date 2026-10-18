# Native pending setup recovery

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), under the existing
[Node decision](../../../docs/architecture/decisions/059-node-foundation.md).
Verified October 18, 2026 with synthetic credentials and owned temporary cores.
No actual Keychain, provider account, startup registration or app removal was used.

`pendingSetup()` acquires the private file lease (creating its empty lock file
when absent) and returns validated installation/setup/device identity, pinned
endpoint, phase and offer expiry. It makes no credential or host call.
`cancelPendingSetup(identity)` persists canceling intent before contacting the
host. Completion cannot resume while cancellation is pending. Exact confirmed
proof is persisted before credential cleanup, so a reopened client can finish
local cleanup without another host request.

Ready-phase token hashes remain binding; a changed token is preserved. Interrupted
preparation binds any retained canonical token before deletion. Missing tokens
are allowed; Keychain read failures are not interpreted as absence. Cleanup checks
the exact token again, verifies absence, removes an exact partially saved connection
when present and removes pending metadata last. Connection cleanup first records
confirmed removal and the retained-history reference. Malformed, unsafe or foreign
history references remain untouched. No pending record returns nil rather than
claiming new revocation. History snapshots and unrelated credentials remain intact.
A foreign history reference requires reconciliation before local cleanup can finish;
this implementation deliberately preserves it rather than choosing which history to
replace. Ordinary app composition and that user-facing recovery remain later work.

## Behavior and failures

- The first public test failed because inspection/cancellation methods were absent:
  `/tmp/fort059-owner-setup-recovery-red.log`.
- Initial test-harness compilation errors involved missing `try` in Testing macros,
  throwing constructors and ambiguous throwing closures. They were corrected before
  behavioral passes. Logs: `...-green.log`, `...-credentials.log`, `...-boundaries.log`.
  The final `...-green.log` was later reused for the 15-function recovery run; the
  initial compile failure is recorded here rather than claimed as a retained log.
- An initial 14-function parallel run failed with `.busy` in the timing test:
  `/tmp/fort059-owner-setup-recovery-tests.log`. An isolated run and unchanged repeat
  passed. A deterministic public cancellation regression then duplicated the held
  lease descriptor and reproduced inability to inspect after completion. macOS
  retains a flock through that duplicate when the owner only closes its descriptor.
  The lease now explicitly unlocks before closing (retrying interrupted unlocks).
  The regression verifies exclusion while active and immediate reacquisition while
  the duplicate is still open. Logs: `...-lease-red.log`, `...-green.log`.
  This proves the lock-lifetime defect and correction; the initial intermittent
  failure did not capture the duplicate and its precise cause remains inferred.
- A new parameterized test exposed replacement of another pairing's saved removal
  or history reference during interrupted finalization. Both identities are now
  checked before either marker is written or a token is deleted. Red evidence:
  `/tmp/fort059-owner-setup-recovery-foreign-red.log`; green in the full suite.
- **127 Swift test functions passed**, including **16 recovery functions** and
  parameterized failure cases: lost/unavailable/foreign receipts, stale identity,
  wrong endpoint/current connection, absent cancellation capability, missing/changed/
  unreadable tokens, failed or ignored removal, before/after preparation failure,
  read-only directory, changed records during host/credential callbacks, unsafe
  records, overlapping operations, task cancellation, partial connection persistence
  and both client/core restart. `/tmp/fort059-owner-setup-recovery-swift.log`.
- Compiled-core recovery covers a committed pairing after both sides reopen, with
  its credential retained or missing. The original token is rejected afterward and
  the stopped core refuses a new setup. Existing fixture cleanup checks clean exit,
  no forced stop, PID/process-group absence and exact socket removal.

Command from the isolated worktree:

```sh
FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node \
FORT_TEST_CORE_ARTIFACT="$PWD/successor/.build/owner-setup-cancellation-pipe-059-20261018" \
swift test --package-path successor/apple/FortKit \
  --scratch-path /tmp/fort059-owner-setup-xpc-native
```

The retained core remains 26 files / 186,928 bytes; manifest SHA256
`5c7dbb937a80b48ce44e5e922909e08cc9a5f9b2ff120be1986dd031d46c00d5`.
No Node or signed-host source changed; the 892 portable, 14 guardian and four signed
scenario results remain separately recorded prior evidence, not rerun claims.

## Local file-operation timing

MacBookPro18,2 / M1 Max / 64 GiB / 10 CPU, macOS 27.2 (26B5112a), Xcode 27.1
(27A9275), Swift 6.4. Two isolated runs after the final code change, with no
concurrent builds. Each has 20 fresh private directories, no system-cache purge,
real bounded file reads, full file sync, atomic rename, directory sync and unlink.
The same-process host and in-memory credentials exclude real Keychain, XPC and
provider latency. p95 is the nineteenth sorted observation. Both target <=500 ms.
Cancellation timing begins after an uncertain ready setup has been prepared and
covers intent, receipt, credential cleanup and pending deletion; the additional
history-marker writes of interrupted finalization are not measured by this workload.
Preparation timing covers the established successful preparation workload.

Cancellation p95: **18.96575 ms**. Preparation p95: **22.40812 ms**.

```json
{
  "cancellation": {
    "p95Ms": 18.96575,
    "samplesMs": [
      15.606958,
      13.484667,
      18.885625,
      13.696709,
      13.643375,
      13.563292,
      13.752959,
      12.906,
      12.740875,
      14.683041,
      14.755041,
      13.784833,
      14.684084,
      13.451541,
      18.96575,
      18.605416,
      17.883041,
      21.843875,
      16.713625,
      17.579375
    ],
    "budgetMs": 500
  },
  "preparation": {
    "scope": "synthetic credential storage and in-process control; real Keychain/XPC excluded",
    "samplesMs": [
      22.408125,
      16.653416,
      18.444875,
      19.590834,
      18.540625,
      17.380625,
      16.431917,
      19.369167,
      16.451875,
      17.427333,
      18.433375,
      16.705292,
      17.494584,
      17.581667,
      17.200333,
      22.916666,
      17.997958,
      20.978541,
      19.374541,
      20.052542
    ],
    "budgetMs": 500,
    "p95Ms": 22.408125
  }
}
```

Logs: `/tmp/fort059-owner-setup-recovery-budget-final.log` and
`/tmp/fort059-owner-setup-recovery-preparation-budget.log`. Concurrent suite timings
and the earlier isolated run are not substituted for these final samples.

## Build and remaining gates

Fresh unsigned engineering builds passed with `CODE_SIGNING_ALLOWED=NO`:

- Mac: `/tmp/fort059-owner-setup-recovery-mac-build/Build/Products/Debug/FortMac.app`.
- iPhone Simulator: `/tmp/fort059-owner-setup-recovery-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.

Commands used the existing `FortSpec058.xcodeproj`, Debug configuration, schemes
`FortMac` / `FortiOS` and generic macOS / iOS Simulator destinations. Logs are
`/tmp/fort059-owner-setup-recovery-{mac,ios}-build.log`. CoreSimulator remains
incompatible (1171.7.0 installed; 1174.9.2 required); device support was disabled.
No Simulator service or permissions were changed. Neither app was installed or run.
Full and implementation spec gates passed; the exact staged gate is required before
commit. This is library/client source recovery, separate from the prior [signed XPC cancellation proof](owner-setup-cancellation-xpc.md).
Ordinary app setup UI, owned host/startup lifecycle, actual Keychain/provider
inspection, screenshots/accessibility, hardware power-loss behavior, reachable
iPhone transport, TestFlight and physical acceptance remain unrun or incomplete.
