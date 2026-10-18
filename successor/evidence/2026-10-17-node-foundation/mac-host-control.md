# Ordinary Mac host controls

October 18, 2026, on top of `af74f09`. Implements the ordinary Mac controls revision
of [Spec 059](../../../specs/059-modular-node-v1.md) under unchanged-scope
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
No real startup registration, App Group access, provider invocation or account
operation was performed. These are source, synthetic behavior and build results.

## Result and boundaries

The ordinary Mac app now composes `MessengerModel` with `MacHostControl()`. Public
construction always obtains `Bundle.main.bundleURL` and is inert. The installed
`SMAppService.h` specifies that an agent plist belongs to the calling app's
`Contents/Library/LaunchAgents`; an arbitrary app URL is therefore not a valid
production verification target. Alternate URLs and backends are internal fixture
injection only. Explicit debug/UI fixtures retain their separate construction.

Inspection verifies the bundle before reading OS registration status. The first
verified client/helper/team/group identity remains pinned even if subsequent status
initialization fails. Enable is a separate action, repeats bundle verification and
uses the existing asynchronous startup boundary. Enabled, approval-required,
not-registered, not-found and unsupported states remain distinct. Registration
does not prove runtime health, pairing, provider access or provisioning eligibility.
Inspection/restore never registers; there is no bare disable control that could
bypass durable removal.

The model permits owner setup only after a verified enabled registration, repeats
inspection before setup/cancellation, and refuses saved foreign connection identities
before credential lookup or host contact. Any host-removal record blocks enable,
including malformed or already-confirmed records awaiting explicit reinstall
reconciliation. Enable holds the existing private progress lease across its action
and checks records again. Failed checks retain records/credentials. Confirmed setup
cancellation can still finish its local cleanup without claiming an enabled host.

The ordinary Mac connection screen shows separate host status/check/enable and
pairing controls. An incomplete engineering package displays unavailable; it does
not substitute the old fixture enrollment path. Existing fixture/recovery launch
packets and their source pins were not changed or reactivated. SwiftUI controls
have accessibility identifiers, but no installed accessibility/screenshot claim
is made. The app was compiled, not launched in this slice.

## Failures and fixes

- `/tmp/fort059-host-control-red.log`: initial missing-controller red. The first
  implementation exposed invalid combined actor-property assertions in the test;
  those assertions were split (`...-first-green.log`).
- `...-green.log` and `...-fresh.log`: strict Swift compilation failed in the
  existing `WorkspaceModelTests.swift` asynchronous URLProtocol fixture, including
  a fresh scratch build. Explicit captures alone still failed (`...-capture-fix.log`).
  An explicitly `@Sendable` closure capturing its request value and already-Sendable
  protocol owner compiled (`...-sendable.log`). No compiler check was disabled,
  new unchecked conformance added, or production transport changed. The complete
  native suite subsequently exercised this fixture successfully.
- `...-sendable.log`: the initial inert/enable behavior passed, but the new identity
  regression failed because status initialization could discard a verified identity.
  Retaining identity before constructing the startup backend fixed it;
  `...-identity-green.log` passed both functions.
- `...-model-red.log` / `...-model-green.log`: the new model fixture's closure
  isolation prevented compilation. Callback isolation is now explicitly MainActor;
  expensive real signature work remains detached inside `MacHostBundle.inspect`.
  `...-model-actor.log` passed the enabled-host/credential behavior. This initial
  model red was a compile failure, not an observed behavioral failure.
- `...-removal-message-red.log`: both saved-removal timing variants proved that
  checking saved setup could overwrite the blocked-state explanation. Keeping that
  explanation while local host setup is blocked fixed both cases;
  `...-removal-message-green.log` passed.
- `...-app-scope-red.log`: the new no-argument construction test failed because the
  API still required an arbitrary URL. Production construction now selects only
  the running app. The final full native suite passed this regression.

All abbreviated log names above start with `/tmp/fort059-host-control`.

## Passed checks

- Seven controller behavior functions cover inert construction/inspection, only the
  running app in public construction, first verified identity retention, signature/
  identity/app changes before enable, five OS statuses, approval-required behavior,
  concurrent enable refusal and cancellation followed by a fresh check.
- Three additional model functions cover no credential before fresh enabled-host
  verification, host-removal records before or after inspection, and saved foreign
  identities before credential lookup. The removal function has two cases.
  All earlier setup behaviors remain passing.
- Final full native suite: **210 executed functions passed** in
  `/tmp/fort059-host-control-final-native.log`. Four prior timing functions, the
  bundle metadata/signature opt-ins and this controller timing opt-in were excluded.
  The earlier pre-API-correction full run passed 209 functions
  (`/tmp/fort059-host-control-native.log`).
- Strict TypeScript plus structural spec checks passed:
  `/tmp/fort059-host-control-typecheck.log`. Portable runtime sources are unchanged;
  their complete tests were not repeated.
- Mac Debug, Mac Release and generic iPhone Simulator Debug builds passed with
  `CODE_SIGNING_ALLOWED=NO`. Each began in a new derived-data directory, then was
  incrementally rebuilt after the final running-app API change. Final logs:
  `/tmp/fort059-host-control-mac-final-build.log`,
  `/tmp/fort059-host-control-release-final-build.log`,
  `/tmp/fort059-host-control-ios-final-build.log`.

Artifacts:

- `/tmp/fort059-host-control-mac-build/Build/Products/Debug/FortMac.app`
- `/tmp/fort059-host-control-release-build/Build/Products/Release/FortMac.app`
- `/tmp/fort059-host-control-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`

Native command uses `swift test --package-path successor/apple/FortKit
--scratch-path /tmp/fort059-host-control-native`, the same synthetic
`FORT_TEST_GUARDIAN`, `FORT_TEST_NODE` and `FORT_TEST_CORE_ARTIFACT` values as the
[preceding bundle slice](mac-host-bundle.md), and `--skip` for the seven functions
listed above. The final app build command uses
`xcodebuild -project successor/apple/FortSpec058.xcodeproj -scheme <FortMac|FortiOS>
-configuration <Debug|Release> -destination <platform> -derivedDataPath <directory>
CODE_SIGNING_ALLOWED=NO build`.

## Controller overhead

Twenty serial inspections on one controller, with synthetic bundle/status backends,
no real signature/filesystem/ServiceManagement work and no concurrent verification.
No registration was called. Same recorded M1 Max 10-core/64 GiB, macOS 27.2,
Xcode 27.1/27A9275 and Swift 6.4 environment as the preceding slice. This measures
controller/actor overhead only; it is not installed startup latency.

Target p95 <=50 ms; nearest-rank p50 **0.015917 ms**, p95 **0.023875 ms**, maximum
**0.128542 ms**. Raw milliseconds:

```json
[0.128542,0.023417,0.022042,0.018458,0.021833,0.017459,0.015083,0.014625,0.015917,0.015125,0.014583,0.023875,0.022,0.018084,0.021291,0.014583,0.014,0.014833,0.013917,0.014167]
```

`FORT_HOST_CONTROL_BENCHMARK=/tmp/fort059-host-control-budget.json swift test
--package-path successor/apple/FortKit --scratch-path /tmp/fort059-host-control-native
--filter hostControlSyntheticInspectionBudget` passed separately.
Log: `/tmp/fort059-host-control-budget.log`.

## Remaining gates

The full production helper/package, exact identifiers/profiles, actual startup
approval, complete owned-installation removal/history/reinstall composition,
provider bootstrap/model discovery and reachable iPhone transport remain separate
work. [Apple activation](../../APPLE-HOST-ACTIVATION.md) is still unperformed.
Available tools still expose no native UI/accessibility automation or authenticated
Apple account inspection. No app was installed/launched, no provider or credential
was accessed, no startup item enabled and no archive/upload/TestFlight occurred.
Historical donor attribution, provider-cleanup limits and unrelated work are intact.
