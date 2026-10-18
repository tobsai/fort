# Authenticated Mac helper process observation

Spec: [059](../../../specs/059-modular-node-v1.md). This is the next local removal
slice after `a447effd5d9fa4af0790bb7d2ee915c405e0d632`, under the unchanged Node v1
decision. It does not activate a production host or complete removal.

## Behavior

`MacXPCTransport.recoverPairingAndObserveHost` returns a validated recovery receipt
and a separate `MacHostProcess` observation. The original `recoverPairing` method
continues to return the receipt alone. Recovery is still the existing bounded owner
control selector; no public HTTP route or new server authority was added.

The client takes PID/effective-user metadata from its code-pinned NSXPC connection
before invalidating it. Every successful XPC reply now requires a nonself PID greater
than one and the current effective user. OS process inspection binds the observation
to that process's start timestamp. No process ID, user ID or host object in reply JSON
can supply this observation. Its public type has no public initializer or decoder.
It is deliberately an in-memory value, not durable recovery evidence.

`hasExited()` queries only the recorded PID. `waitForExit()` accepts an explicit
finite positive deadline up to 30 seconds, sleeps at most 25 ms between checks and
performs no work while idle. ESRCH or a different start timestamp confirms that the
original process is absent. An unchanged timestamp remains live even if its user ID
changes. Permission and other inspection errors propagate as unconfirmed; short or
malformed OS results cannot establish absence. Cancellation and timeout affect only
the waiting caller. This code has no process signal, launch, startup registration,
credential removal, directory cleanup or provider operation.

The SDK's `NSXPCConnection.h`, `libproc.h` and `sys/proc_info.h` were inspected. Apple's
[peer PID documentation](https://developer.apple.com/documentation/foundation/nsxpcconnection/processidentifier)
and [XNU process inspection source](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/proc_info.c)
support the metadata/lookup distinction. Actual compatibility with the existing
App Sandbox permissions was tested below; source inspection alone is not that proof.

## Verification on October 18, 2026

Environment: MacBookPro18,2, M1 Max, 64 GiB, 10 CPU; macOS 27.2 (26B5112a),
Xcode 27.1 (27A9275), Swift 6.4, Node 26.5.0, TypeScript 6.0.3.

Passed:

- Five new native behavioral functions: live process/timeout/exit/repeated wait;
  independent concurrent wait and cancellation; invalid peer/deadline and absent
  initial lookup; process-start replacement and same-process effective-user change;
  inspection errors followed by explicit successful retry. Actual children are
  exact `/bin/cat` processes with empty environments and private stdin pipes. Tests
  close their pipes and reap them; the observer does not terminate them. Replacement
  and permission-error cases use an injected OS-result seam, not a real PID reuse
  or privilege change.
- Full FortKit suite: **178 executed test functions passed**. Three isolated timing
  functions were excluded. The new timing function then passed separately; the two
  unchanged startup/storage timing functions retain their earlier evidence.
- The signed embedded-XPC regression passed: **one scenario, zero skips**, 39.111 s
  including compilation/signing and its existing crash/restart journey. It used
  development identity `58F40D1AB480565B89109B853235AC3A7B2E7AD1`, team `T3JB5MYZ93`.
  Parent sandbox and helper/child entitlements were unchanged. It verified exact
  observed helper PID against independent fixture diagnostics, deliberately forged
  reply PID/user/host fields, refusal of partial/foreign/malformed receipts, live
  timeout without termination, and OS-confirmed core-helper exit within three
  seconds. Both signed-peer mismatch directions and disabled owner control still
  fail. Existing authentication, setup/cancel/restart/crash, outside-file/TCP/outer
  executable denials and exact process/group/container cleanup assertions passed
  without forced cleanup. The successful temporary artifact was removed by the
  test; this is not a retained or installed product artifact.
- Fresh unsigned Debug Mac and iPhone Simulator client builds passed. Artifacts:
  `/tmp/fort059-host-process-mac-build/Build/Products/Debug/FortMac.app` and
  `/tmp/fort059-host-process-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.
  Build logs: `/tmp/fort059-host-process-mac-build.log` and
  `/tmp/fort059-host-process-ios-build.log`. Neither app was launched or installed.
- Strict TypeScript typecheck, full structural spec gate, accepted implementation
  gate and whitespace check passed. Staged-path coverage is checked before commit.

Commands/logs:

- `swift test --package-path successor/apple/FortKit --scratch-path /tmp/fort059-owner-setup-xpc-native --filter hostProcess --skip hostProcessInspectionBudget`
  — `/tmp/fort059-host-process-tests-pass.log`.
- Full native command uses `FORT_TEST_GUARDIAN=/tmp/fort059-mac-core-host-guardian`,
  `FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node` and the retained
  `successor/.build/owned-core-059-20261018` artifact, excluding the three timing
  functions — `/tmp/fort059-host-process-native.log`.
- `FORT_SIGNING_IDENTITY=58F40D1AB480565B89109B853235AC3A7B2E7AD1 /opt/homebrew/opt/node@26/bin/node --test --test-name-pattern='embedded XPC peers' successor/packaging/sandbox-feasibility.test.ts`
  — `/tmp/fort059-host-process-xpc.log`.
- `PATH="/opt/homebrew/opt/node@26/bin:$PATH" npm --prefix successor run typecheck`
  — `/tmp/fort059-host-process-typecheck.log`.

## Inspection cost

Twenty serial inspections of one live synthetic process, with warm binary/kernel
metadata caches and no concurrent verification work. Each sample includes only the
synchronous OS lookup and identity comparison. Clock-read overhead is included;
these very short samples are close to clock/measurement overhead. No provider,
process launch/shutdown, XPC, signing, UI, compiler or disk work is included.

Target p95 <=5 ms; nearest-rank p50 **0.000375 ms**, p95 **0.000916 ms**. This is not
an installed-host or cold-start benchmark. The bounded explicit wait is not an idle
poller, and no per-process memory or wakeup result is inferred from this measurement.

Raw milliseconds:

```json
[0.000958,0.000916,0.000417,0.000416,0.000375,0.000417,0.000375,0.000375,0.000333,0.000375,0.000375,0.000375,0.000375,0.000375,0.000375,0.000375,0.000375,0.000375,0.000375,0.000416]
```

Command: `FORT_HOST_PROCESS_BENCHMARK=/tmp/fort059-host-process-budget.json swift test --package-path successor/apple/FortKit --scratch-path /tmp/fort059-owner-setup-xpc-native --filter hostProcessInspectionBudget`.
Log: `/tmp/fort059-host-process-budget.log`.

## Preserved failures and limits

The initial sandboxed Swift invocation could not write the normal compiler cache
(`/tmp/fort059-host-process-red.log`). Supported escalation was approved. The first
usable red failed because `MacHostProcess` was absent
(`/tmp/fort059-host-process-missing-red.log`); the minimal behavior then passed in
`/tmp/fort059-host-process-first-green.log`. Expanded tests initially failed to
compile because throwing expressions inside Swift Testing macros needed explicit
`try` placement (`/tmp/fort059-host-process-tests.log`); corrected test syntax passed.

The signed red failed during compilation because the new public XPC operation did
not exist (`/tmp/fort059-host-process-xpc-red.log`). Its incomplete artifact remains
at `/tmp/fort059-sandbox-xpc-test-CUElku`; it never reached execution. The existing
signing identity, entitlement policy and process bounds were not weakened.

The 900 portable Node tests and unchanged non-XPC/Codex packaging scenarios were
not rerun for this Swift/XPC-only product change. Prior evidence remains separate.
The compiled core artifact is unchanged: 30 files, 190826 bytes, manifest SHA256
`c1a19bd8e8f0def68efa45f2b913dc5b2123466a3da6a5c14ce0eb8fded7f527`.

This observation does not prove startup unregistration, identify a later restarted
helper, survive a client restart, or authorize credential cleanup. Production removal
still needs durable partial-progress records, exact registration control and the
verified package/profile boundary. The signed fixture's delayed exit is synthetic;
production must wait for actual registration removal, not copy that delay heuristic.
No actual SMAppService call, App Group lookup, credential access, provider, installed
UI/accessibility/screenshot, physical device, archive, upload or TestFlight action
ran. CoreSimulator remains older than the installed Xcode requirement.
