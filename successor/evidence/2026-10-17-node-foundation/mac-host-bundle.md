# Native host bundle inspection

Local source verification on October 18, 2026, on top of `38adc02`. This implements
the native host bundle revision of [Spec 059](../../../specs/059-modular-node-v1.md)
under unchanged-scope [decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
It does not activate a host or establish provisioning, provider or TestFlight readiness.

## Behavior and trust boundary

`MacHostBundle.inspect(applicationURL:)` reads the exact app, fixed nested helper,
identical bounded configuration and exact LaunchAgent definition. It refuses unsafe
paths/files, missing executable permission, extra startup behavior and inconsistent
identities. Both independent app signatures must satisfy the Apple-anchored exact
identifier/team requirement before any signing metadata is used. Strict resource,
nested-code and all-architecture verification is enabled. Both apps must declare
App Sandbox, the sole configured App Group and no sandbox-inherit entitlement.

Each app needs its own nonempty regular embedded profile, bounded to 1 MiB.
`provisioningEvidence` explicitly reports `presentUnverified`: profile bytes are
not decoded or authenticated here. Apple eligibility, supported entitlements and
actual activation still require the [separate gate](../../APPLE-HOST-ACTIVATION.md).
Offline signature checks disable network access and do not prove current online
revocation or notarization status. Static inspection does not protect against
concurrent same-user bundle replacement; repeat it before an activation action.

Inspection runs off the UI actor and creates no idle process. It does not call
App Group lookup, ServiceManagement, Keychain, XPC, a runtime or a provider.
The agent-host boundary remains distinct from any computer-tool sandbox.

The implementation follows the installed SDK's `SecCode.h`, `SecStaticCode.h`
and `CSCommon.h`: signing information alone is insufficient, successful validity
checking must precede its use, and static validation assumes no concurrent mutation.
The bundle-relative LaunchAgent layout follows Apple's
[SMAppService documentation](https://developer.apple.com/documentation/servicemanagement/smappservice)
and [helper migration guidance](https://developer.apple.com/documentation/servicemanagement/updating-helper-executables-from-earlier-versions-of-macos).

## Test-first record and corrections

- `/tmp/fort059-host-bundle-red.log`: expected red, missing public inspection types.
- `/tmp/fort059-host-bundle-first-green.log`: compile failures for the imported
  network-access flag and comparing `timespec` values. Use `.noNetworkAccess`
  and compare timestamp seconds/nanoseconds explicitly.
- `/tmp/fort059-host-bundle-green.log`: fixture URL equality failed because a
  directory URL gained a trailing slash after creation. Explicit directory URLs
  corrected both representations; the first behavior passed in
  `/tmp/fort059-host-bundle-first-pass.log`.
- `/tmp/fort059-host-bundle-launch-red.log`: numeric MachServices value was refused
  but reported as an entitlement failure. The launch-definition boundary now
  returns its own error before signature checks. Both initial behaviors passed
  in `/tmp/fort059-host-bundle-launch-green.log`.

Final synthetic coverage has nine executed functions and 53 cases: valid unchanged
metadata; numeric Mach value; 11 configuration mutations; 9 alternate startup
definitions; 9 unsafe layouts (including a FIFO opened nonblocking); 8 missing,
empty, oversized or linked profiles; 12 independent signature/entitlement failures;
fresh refusal after metadata changes; and the public async API rejecting an unsigned
bundle. The two explicit opt-in functions were skipped in this targeted run, then
executed separately. Log: `/tmp/fort059-host-bundle-behavior.log`.

## Actual signature verification, without execution

The existing authorized signed artifact was inspected read-only:
`successor/.build/codex-version-059-20261018/FortSandboxProbe.app`.
Exact identifier `io.fort.foundation.probe.43249c4a-5fd2-4810-946d-6e0060d70e81`,
team `T3JB5MYZ93`. The real Security backend accepted it and a fresh temporary copy,
rejected a different identifier and team, and rejected a modified sealed resource
in that copy. The original resource was verified unchanged; the copy was removed.
No executable was launched or re-signed, and no account was accessed.

The one final opt-in function passed in 1.851 seconds, including copy, repeated
signature checks and cleanup. That is an incidental test duration, not a benchmark.
This artifact has no production App Group/helper/profile configuration and is not
a full `MacHostBundle.inspect` success. Logs:
`/tmp/fort059-host-bundle-signature.log` and
`/tmp/fort059-host-bundle-signature-tamper.log`.

Command uses `FORT_TEST_SIGNED_HOST_BUNDLE`, `FORT_TEST_SIGNED_HOST_IDENTIFIER`
and `FORT_TEST_SIGNED_HOST_TEAM` with the values above, then:
`swift test --package-path successor/apple/FortKit --scratch-path /tmp/fort059-owner-setup-xpc-native --filter hostBundleCodeSignatureVerifiesRetainedSignedProbe`.

## Isolated metadata cost

Twenty serial inspections of one synthetic app/helper fixture, warm filesystem
caches, an injected signature reader and no concurrent verification work. Includes
opening and validating layout/manifest/LaunchAgent/profiles, not actual signature
verification, profile eligibility, startup, provider work or UI. The fixture contains
five configuration fields, three LaunchAgent fields, two small synthetic profiles
and one synthetic executable. Hardware/toolchain is the previously recorded M1 Max,
10 CPU cores/64 GiB, macOS 27.2, Xcode 27.1/27A9275, Swift 6.4 environment; a repeat
`sysctl` inventory was sandbox-denied, so no fresh hardware inference is claimed.

Target p95 <=50 ms. Nearest-rank p50 **0.510167 ms**, p95 **0.556625 ms**, maximum
**1.368875 ms**. The proposed full signed production-host p95 <=2 seconds remains
unmeasured because that artifact does not yet exist.

Raw milliseconds:

```json
[1.368875,0.556625,0.52775,0.524875,0.518167,0.507916,0.511792,0.513042,0.508583,0.510167,0.504333,0.505459,0.501208,0.511083,0.501041,0.507125,0.528709,0.50575,0.513416,0.50675]
```

Command: `FORT_HOST_BUNDLE_BENCHMARK=/tmp/fort059-host-bundle-budget.json swift test --package-path successor/apple/FortKit --scratch-path /tmp/fort059-owner-setup-xpc-native --filter hostBundleMetadataInspectionBudget`.
Log: `/tmp/fort059-host-bundle-budget.log`.

## Regression/build checks and remaining work

- Full native suite: **200 executed functions passed**, with the four historical
  timing functions, new metadata timing function and opt-in signature function
  excluded. Existing synthetic guardian/Node/core artifact environment matches
  [the preceding native removal run](mac-host-removal.md). Log:
  `/tmp/fort059-host-bundle-native.log`.
- `PATH="/opt/homebrew/opt/node@26/bin:$PATH" npm --prefix successor run typecheck`
  passed, including the full spec gate. Log: `/tmp/fort059-host-bundle-typecheck.log`.
- Fresh `xcodebuild` Debug builds with `CODE_SIGNING_ALLOWED=NO` passed for FortMac
  and FortiOS (generic iOS Simulator), using `successor/apple/FortSpec058.xcodeproj`.
  Artifacts: `/tmp/fort059-host-bundle-mac-build/Build/Products/Debug/FortMac.app`
  and `/tmp/fort059-host-bundle-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.
  Logs: `/tmp/fort059-host-bundle-{mac,ios}-build.log`.
- Full structural and accepted implementation gates passed. Unchanged portable
  runtime tests were not repeated for this Swift-only slice; their last evidence
  remains [the Codex executor run](codex-executor.md).

No installed UI/accessibility/screenshots, real provider, App Group lookup, startup
registration, production provisioning, archive, upload or TestFlight acceptance was
performed. Ordinary app/host lifecycle integration and helper packaging remain local
implementation work. Actual activation is separately blocked on the exact signed
package, approved identifiers/profiles and persistent-startup authority. No provider
cleanup, new entitlement grant, credential, startup service or deployment occurred.
