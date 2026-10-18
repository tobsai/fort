# Native companion authority and removal

Date: 2026-10-18. Base: `0d75bf1`. Governed by the native companion refinement in
[Spec 059](../../../specs/059-modular-node-v1.md) under decision 059. Source and
synthetic native execution are verified here. Actual phone setup, installed controls,
credential access, secure-network activation and TestFlight acceptance are not.

## Behavior

The shared Swift client retains the service's companion role. On installations
advertising companion support, host controls require a successfully validated pairing
status. A phone cannot invoke native stop/resume/restart. Its disconnect sends only
the exact companion leave request; response installation, device, revocation and
companion-only scope must all match. Server enforcement remains independent of UI.

Before sending, the existing removal journal records companion scope. Lost replies
retain the exact credential and history; relaunch retries the same leave operation.
No phone recovery calls the Mac host's private recovery or stop endpoint. After the
response, changed connection or intent prevents local credential removal. Confirmed
partial credential cleanup resumes locally without another remote request. Local
forgetting remains unconfirmed and directs revocation to the owning Mac. Legacy
records without scope retain their existing host semantics.

Saved-history deletion accepts either scope only after exact confirmed revocation,
connection absence and credential absence. History remains by default. Settings
distinguish phone leave from whole-host disconnect and hide host controls for a
companion. These view changes have compile/model proof, not installed screenshot or
accessibility signoff. The earlier transcript QA result belongs to its earlier build.

## Reproduced failures and verification

- Role test first failed because Swift discarded `role: companion`; fixed with the
  optional typed role, preserving legacy decoding.
- Native disconnect tests first called `/v1/pairing/disconnect` and claimed runtime
  shutdown. They now call only `/v1/pairing/leave`, with retained phone scope on retry.
- A confirmed phone removal initially failed saved-history deletion because its
  scoped intent differed from the old host-only equality check. Exact identity,
  confirmed state and credential absence now govern that operation.
- A late response initially overwrote a changed removal intent. Both changed
  connection and changed-intent tests now keep the current records and credentials.
- An intermediate compile failed for a missing `try` on a second record read. The
  correction reads that record once. This compile failure was separate from the
  behavioral reds above.
- Focused final lifecycle run: **11 test functions passed**, including parameterized
  receipt/authority/state-change cases and existing host stop/resume/lost-reply tests.
- Full native run: **269 functions passed**, **11 optional skips**, zero failures;
  the runner reports 280 including skips, 52.135 seconds. Nine skips are isolated
  budgets; two are retained signed/review-package probes whose inputs were not set.
  The affected new budget passed separately below. Existing startup/host-history
  synthetic journeys ran with fresh compiled resources.
- Fresh unsigned **FortMac Debug** and **FortiOS Simulator Debug** builds passed.
  No installer, signing account, entitlement grant or device launch was used.

Native command (existing Xcode 27.1 / Swift 6.4):

```sh
FORT_TEST_HOST_RESOURCES=/tmp/fort059-native-companion-host-resources \
FORT_TEST_CORE_ARTIFACT=/tmp/fort059-native-companion-host-resources/core \
FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node \
FORT_TEST_GUARDIAN=/tmp/fort059-assembly-guardian \
xcrun swift test --package-path successor/apple/FortKit \
  --scratch-path /tmp/fort059-maintenance-swift --no-parallel
```

Builds use `successor/apple/FortSpec058.xcodeproj`, scheme `FortMac` with destination
`generic/platform=macOS`, or `FortiOS` with `generic/platform=iOS Simulator`, Debug,
`CODE_SIGNING_ALLOWED=NO`, and the separate output directories below. No custom build
script phases were present. Simulator execution remains unavailable with installed
CoreSimulator 1171.7.0 versus Xcode's required 1174.9.2; no system update was attempted.

The model fixtures run on Mac through the existing injected transport using a
synthetic endpoint. They exercise shared Swift behavior, not real XPC, physical iOS,
URLSession trust or Keychain. Disposable credential values stay in memory. The
ordinary HTTPS transport's platform trust and redirect refusal are unchanged.

## Isolated performance and artifacts

MacBookPro18,2, 10 CPUs, 64 GiB, macOS 27.2. Five warmups and 20 sequential samples,
no simultaneous build/test workload. Measure leave, lost reply, model relaunch and
exact retry, including private file operations. Fixture construction/initial restore
is outside timing; transport and credential storage are synthetic. No provider or
network cost is included. No new polling/timer is added.

`FORT_NATIVE_COMPANION_BENCHMARK=/tmp/fort059-native-companion-benchmark.json xcrun swift test --package-path successor/apple/FortKit --scratch-path /tmp/fort059-maintenance-swift --no-parallel --filter nativeCompanionLostReplyRecoveryMeetsItsIsolatedBudget`

p50 **1.895 ms**, p95 **2.112 ms**, maximum **2.294 ms** against a 100 ms p95 budget.

Fresh `/tmp/fort059-native-companion-host-resources` contains 98 files, 413,050 bytes;
every manifest size/hash was independently verified. The optional executor is
compiled only; no actual Codex binary or provider was invoked. The core is unchanged
from the companion service commit's independently tested artifact. Portable/strict
TypeScript results remain those in [service evidence](companion-pairing.md); this
native-only slice did not repeat unaffected TypeScript tests.

| Evidence | SHA256 |
| --- | --- |
| Full native log `/tmp/fort059-native-companion-full.log` | `41cf5bc9cd84a73f04d3a316377b3afca716563fb1b0d20e15ffeddbe2a70641` |
| Isolated timing JSON | `cbbcad9b69d2a7e6ed14a41cda2fa12d1ec137276320ee1910452cad8e6b7686` |
| Host resource manifest | `85ee898362cd0dbf7061bb7c4c748a1a441bbce4533bcb98be9d9a5ec96c496e` |
| Both app inventories `/tmp/fort059-native-companion-artifacts.json` | `36a9300c6698dad481f0997dad373942aa0d49f7ab8e1b47b98fe0b1e3e9829d` |
| Mac 7-file inventory | `5bfae9e0a03d8372cfbadcb73cebb38aa6a82b8ff0f6469f66545e191f1007bc` |
| iPhone Simulator 7-file inventory | `212d058b9efbf292ad2e6af2d02d8f8bf1258f0cf9d648214d561c28cdc1d243` |

Apps: `/tmp/fort059-native-companion-mac/Build/Products/Debug/FortMac.app` and
`/tmp/fort059-native-companion-ios/Build/Products/Debug-iphonesimulator/FortiOS.app`.
Neither was launched. No new actual credential, permission, startup registration,
trust installation, account sign-in, provider session or deployment occurred.

## Remaining acceptance

Native invitation/prepared-phone credential setup and expired-phone restoration
still need composition. Secure-listener lifecycle and production TLS trust remain
separate, explicit steps. Apple record verification is waiting on the existing
account sign-in. Current settings need exact installed accessibility/screenshots;
physical iPhone/Mac, real-provider, App Group/Keychain/startup and TestFlight gates
remain open. This is a bounded source slice, not v1 completion.
