# Explicit Mac startup registration boundary

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), under the existing
[Node decision](../../../docs/architecture/decisions/059-node-foundation.md).
Verified October 18, 2026 with a synthetic registration backend. No real
ServiceManagement status/register/unregister call, startup grant, service activation,
provider access or credential operation was exercised by these tests.

`MacHostStartup` takes a validated agent endpoint and derives its exact
`<serviceIdentifier>.plist`, rejecting a name over 255 bytes. Construction performs
no lookup or registration. Its system adapter lazily creates `SMAppService.agent`
and owns it on an actor, keeping synchronous framework calls off the UI actor.
The adapter is compiled but has not been invoked against an installed helper.
The ordinary app and owned-installation adapter do not construct this wrapper yet.

The public state distinguishes unknown, not registered, eligible/enabled,
approval needed, not found and unsupported. Inspection is explicit and read-only.
Enable first inspects current state: an existing enabled/approval-needed job is
returned without another registration; missing/unsupported state is refused.
Disable awaits the asynchronous unregister callback and then checks current state.
After an error, both actions reconcile observed state before deciding whether the
registration-only goal was reached. Repeated calls and reopened wrappers converge
without compensating registration or removal.

All actions, including inspection, expose a pending operation and reject overlap.
An early backend status change cannot complete the public action before its callback.
Task cancellation before dispatch makes no call. Cancellation after dispatch waits
for completion and refreshes the observed state before throwing; it does not undo
an already-applied registration change. If the OS callback never returns, the
operation stays pending. No timeout is presented as success and no timer/polling or
launchctl command was added.

Registration state is not process state. Enabled means eligible to run; unregistered
means no future registration launch. Neither proves runtime stop, pairing revocation,
helper/descendant absence or whole-app removal. The future owned-installation adapter
must combine independent proofs before deleting credentials. Profile/bundle and
activation gates remain in [the agent boundary evidence](mac-agent-boundary.md).
The SDK callback distinction is documented by
[SMAppService](https://developer.apple.com/documentation/servicemanagement/smappservice)
and the installed Xcode 27.1 `SMAppService.h` header.

## Verification

- Red: missing startup types/backend/public action, then missing disable method.
  `/tmp/fort059-mac-host-startup-red.log` and `...-disable-red.log`.
- First green: explicit inspection/enable only; repeated enable makes one registration.
  `...-first-green.log`.
- Disable green: delayed callback stays pending and rejects inspect/enable/disable
  overlaps; repeated completed removal issues no additional request.
  `...-disable-green.log`.
- Eleven initial startup functions passed in `...-boundaries.log`. The final suite
  adds cancellation after register dispatch, including no compensating removal.
- **156 Swift behavioral functions executed and passed**, with **one explicit timing
  function excluded** from that parallel run. Swift Testing reports 157 registered
  functions, including the excluded one. `/tmp/fort059-mac-host-startup-swift-pass.log`.
  Startup coverage includes external approval revocation, absent/unsupported states,
  before/after/ignored registration/removal failures, reopen/retry, cancellation
  before dispatch and after either mutation, cancellation during initial status
  inspection, overlap rejection and invalid plist names.
- The timing function ran separately with `FORT_STARTUP_BENCHMARK=1` and passed.
  `/tmp/fort059-mac-host-startup-budget.log`.

Full-suite command from the isolated worktree:

```sh
FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node \
FORT_TEST_CORE_ARTIFACT="$PWD/successor/.build/owner-setup-cancellation-pipe-059-20261018" \
swift test --package-path successor/apple/FortKit \
  --scratch-path /tmp/fort059-owner-setup-xpc-native
```

For the isolated timing run, add `FORT_STARTUP_BENCHMARK=1` and
`--filter macHostStartupSyntheticInspectionMeetsLocalProjectionBudget`.
No core/host-channel/signed-package source changed. Prior 892 portable, four signed
sandbox scenarios and guardian checks remain separate evidence, not rerun claims.

## Measurement and preserved failure

The first full parallel run failed only its timing assertion: p95 **122.712417 ms**
against 50 ms. `/tmp/fort059-mac-host-startup-swift.log`. Its asynchronous actor call
waited alongside unrelated tests on the UI actor, including synchronous durable-file
work. That contended run measures queue delay as well as this projection and is
preserved; it was not relabeled a pass or used to increase the budget. The spec/test
now require an explicit isolated timing run, with normal suite execution reporting
that one timing exclusion. Actual loaded UI responsiveness remains an installed gate.

Final measurement conditions: MacBookPro18,2 / M1 Max / 64 GiB / 10 CPU, macOS 27.2
(26B5112a), Xcode 27.1 (27A9275), Swift 6.4. Twenty new wrappers inspect the same
synthetic actor backend in the approval-needed state. Construction is excluded;
timing covers UI/backend actor scheduling and status projection, with no other
running tests/builds, no OS ServiceManagement call and no cache purge. p95 is the
nineteenth sorted sample. Target remains <=50 ms.

Measured p95: **0.017042 ms**.

```json
{
  "p95Ms": 0.017042,
  "budgetMs": 50,
  "samplesMs": [
    0.073792,
    0.012959,
    0.009542,
    0.009041,
    0.008917,
    0.008792,
    0.008125,
    0.017042,
    0.016917,
    0.017041,
    0.015791,
    0.0085,
    0.009791,
    0.008667,
    0.00825,
    0.007959,
    0.00775,
    0.007833,
    0.007958,
    0.008
  ]
}
```

## Builds and remaining acceptance

Fresh unsigned engineering builds passed with `CODE_SIGNING_ALLOWED=NO`:

- Mac: `/tmp/fort059-mac-host-startup-mac-build/Build/Products/Debug/FortMac.app`.
- iPhone Simulator: `/tmp/fort059-mac-host-startup-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.

Commands used the existing `FortSpec058.xcodeproj`, Debug configuration, `FortMac` /
`FortiOS` schemes and generic macOS / iOS Simulator destinations. Logs:
`/tmp/fort059-mac-host-startup-{mac,ios}-build.log`. CoreSimulator remains incompatible
(1171.7.0 installed, 1174.9.2 required), with device support disabled. Neither app
was installed or run; no OS setting was changed. Full/implementation spec gates
passed, and exact staged coverage is required before commit.

Real system registration/status
latency, permission and denial behavior, update/rollback, the owned-installation
adapter, helper/runtime/process absence and installed UI remain unverified.
No registered agent, app/profile/grant, TestFlight delivery or physical acceptance
is implied by this source and synthetic test result.
