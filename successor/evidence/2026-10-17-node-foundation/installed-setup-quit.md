# Installed setup Quit and Close repair

October 19, 2026. Spec 059 / decision 059. Base commit
`7a0713642acb921ae54b98bc5273e98095d8c133`. The coordinator explicitly requested
repair of normal Quit and repeated setup/open/close checks without activation.
This evidence supersedes the open Quit issue in the preceding preflight/review.
It does not grant host/provider access or authorize the held G2 CSR upload.

## Root cause and smallest change

The exact installed baseline accepted Quit Fort, but its process remained after
2.335 seconds. The checked-in regression then failed after its five-second exit
allowance: `Normal Quit left the selected client running`. A one-second sample
showed the main thread waiting normally for AppKit events.

A minimal SwiftUI app reproduced the failure with only a sheet, text and buttons.
It quit normally after closing the sheet, with no Fort model, task, storage or
runtime involved. Changing only the sheet window's
`preventsApplicationTerminationWhenModal` to false allowed normal Quit while open.
The [AppKit property](https://developer.apple.com/documentation/appkit/nswindow/preventsapplicationterminationwhenmodal)
and [Apple's guidance](https://developer.apple.com/videos/play/wwdc2026/289/)
explain the modal default and recommend opting out when user intervention is not
required. The local SDK declares this property since macOS 10.6; SwiftUI's related
modifier requires 15.4. Fort's minimum remains macOS 14.

Only MessengerView changed in the product. A private NSViewRepresentable sets the
property on the setup sheet's own window. It cannot receive clicks or accessibility
focus. No global termination override, process exit hook or other sheet policy was
added. A Close setup toolbar action replaces the conditional Return to conversation
action and only changes presentation state. It leaves pending durable setup,
credentials, host lifecycle and the no-default-expiry pairing policy intact.
Connection received an accessibility identifier. [Final setup window](installed-setup-close.png).

## Installed regression result

`successor/apple/check-installed-setup.swift` verifies the selected app's exact
bundle ID/signature, refuses an existing selected process, and uses accessibility
for normal Quit Fort, Close setup and Connection. At this window size, Connection
is in macOS's toolbar overflow, which the test opens explicitly. It never presses
Enable, Connect or sign-in and contains no force-kill operation. It omits text-field
contents. Existing accessibility access is required; it requests no permission.

On the final exact client-only package, three cycles passed:

- Six normal Quits, including three with setup open. Exit observations were
  0.337055, 0.051615, 0.052635, 0.054810, 0.052623 and 0.053124 seconds, all below
  the five-second limit. These are bounded UI checks, not startup benchmarks.
- Nine setup Close/Connection reopen checks. Every setup inspection found Enable
  available and Connect disabled. No activation action was performed.
- Three main-window close/reopen checks, retaining the same process before Quit.
- All test clients exited normally. A final accessibility/screenshot capture was
  followed by another successful normal Quit and independent process absence.

The first fixed-package test exceeded its initial five-second launch-callback
allowance before reaching Quit. Launch gets thirty seconds in the final runner;
the Quit limit remains five. The next attempt passed Quit but did not find a direct
Connection button. Inspection located the toolbar overflow; following that real
menu path completed all cycles. These test limitations were corrected without
further changes to signed product bytes. Earlier baseline failures required scoped
SIGTERM cleanup; no final repaired client required forced termination.

Reproduce from the isolated worktree after verifying the exact package below and
confirming the selected client is stopped:

```sh
xcrun swiftc -parse-as-library -target arm64-apple-macosx14.0 \
  -module-cache-path /tmp/fort059-swift-module-cache \
  successor/apple/check-installed-setup.swift -o /tmp/fort059-check-installed-setup
/tmp/fort059-check-installed-setup /Users/tobiasgunn/Applications/Fort.app 3
```

The actual environment was arm64 macOS 27.2, Xcode 27.1 build 27A9275, English
accessibility labels. macOS 14 compiled compatibility is distinct from runtime
acceptance on an older OS. No physical iPhone or TestFlight execution is claimed.

## Exact packages and other validation

Installed client-only artifact: `/Users/tobiasgunn/Applications/Fort.app`, identical
to `/tmp/fort059-setup-quit-signed-20261019/Fort.app`.

- 206 files, 406,509,282 bytes and 30 Mach-O objects.
- Signed package report SHA256:
  `657aee7bef3665e53978e1dd8c8481ee83e9b8243fd090fe1de136de23ed11a4`.
- Payload manifest SHA256:
  `3fe876908e17ed1f50d68363c564a7c11ff522e8c0018851a6b3730cfad3c32c`.
- Helper still has no outgoing-network entitlement. The separately signed,
  uninstalled/unlaunched provider candidate is pinned in the refreshed
  [one-time authorization proposal](installed-host-proposal.md).

Both packages passed every file membership/mode/size/hash check, all 30 strict
all-architecture signatures and exact approved public signing certificate
fingerprints, nested verification, exact profiles/entitlements and native bundle
inspection. Both unsigned inputs failed signature verification as expected.
Identity/team/group/LaunchAgent definitions and child sandbox inheritance remain
unchanged. Source reports pin MessengerView's SHA256 against the base above.

Release FortMac and FortHost builds passed in 28.651 and 3.124 seconds respectively.
The iPhone Simulator Release build passed. The focused native run had 66 passing
functions, zero failures and two optional timing skips. These use synthetic startup,
credentials and compiled core recovery fixtures. No fresh full portable suite was
needed for this bounded Swift UI change.

Final independent checks confirmed all 206 installed file records unchanged, no
owned client/helper/proposal/diagnostic process, no exact launchd registration and
no App Group/FortHost directory. Native task storage still contains only the
zero-byte owner-setup.lock. No credential content was inspected and no provider
credential, pairing or service activation occurred. The previous installed copy
is preserved at `/tmp/fort059-installed-pre-quit-fix-preserved-20261019.app`.

Raw evidence remains in `/tmp/fort059-setup-quit-red.jsonl`,
`/tmp/fort059-setup-quit-installed-verified.jsonl`,
`/tmp/fort059-setup-quit-native-green.log`,
`/tmp/fort059-quit-candidate-verification.json`,
`/tmp/fort059-setup-quit-final-absence.json`, the final accessibility/screenshot
files, package reports and native build logs. The stopped minimal reproduction
and its two source versions are retained under `/tmp/fort059-quit-diagnosis`.
No diagnostic instrumentation was added to product code.

## Remaining gates

Actual helper registration, effective App Group/XPC, provider account/model/traffic,
stop/resume/unregistration/local credential removal, privacy declarations,
TestFlight and physical acceptance remain unrun. Their exact approvals remain
pending. The 30-minute/six-request proposal is a one-time attended test boundary;
it is not a pairing expiry, runtime quota or product limit. The G2 upload remains
held under the parent's separate approval flow. No upload or certificate issuance
was attempted during this repair.

## Settings sheet repair, October 20

Source base: 6ffca1e0f3047396e8ae7884b391da55e369044d. The installed paired cleanup
later exposed a second sheet with the same AppKit default: Quit with Settings open
left the client running; Back/Done then Quit exited. The October 19 fix above only
attached its window bridge to Connect to Fort. It did not cover Settings.

FortSettingsView now attaches that same bridge. SetupSheetWindow was renamed to
internal NonBlockingSheetWindow so the two views share it. The bridge still only
sets its containing window's preventsApplicationTerminationWhenModal to false when
attached. It cannot receive clicks or appear in accessibility. No global termination
override, account cancellation, host stop, persistent write or dependency was added.
The Settings navigation root keeps the bridge attached while its account page is open.

The new settingsSheetAllowsNormalQuitWithoutChangingDraftOrStoredState test renders
the real Settings view into a hidden NSWindow. Before the fix it fails because the
window blocks termination. Afterward it passes, with the draft unchanged, the window
still invisible and the temporary storage empty. It never orders a window front or
constructs an owned host/provider connection. This checks the public window behavior;
it does not invoke Quit on an installed app or prove preservation across process exit.

Validation:

- Observed red: one new test fails at the modal termination assertion. Green: the
  same test passes after the three-line Settings attachment and bridge rename.
- The focused settings/setup/removal-model run has 25 passing test functions, no
  skips and no failures with `swift test --disable-sandbox --no-parallel`. Two
  earlier parallel runs each have 24 passes and one existing timing failure:
  ordinaryMacRemovalCompositionMetadataBudget p95 52.55 and 65.75 ms versus its
  50 ms limit. The first overlaps native builds; the second does not. Its async
  samples share the main actor with the other tests, including SwiftUI initialization.
  Sequential p95 is 0.668 ms. No production timing code or test limit was changed;
  parallel timing reliability remains an open validation concern before broad CI signoff.
- Fresh unsigned Release FortMac and FortiOS Simulator builds pass. The installed
  CoreSimulator service/version warning prevents simulator device support in this
  environment; these are compile results, not simulator/device execution.
- Full spec, accepted implementation, staged scope/review and whitespace checks pass.
  No fresh portable suite, helper package/signature checks, installed AX/screenshot,
  changed-build Quit, real-provider or TestFlight acceptance ran for this Swift view fix.

Commands and raw logs are under /tmp/fort059-settings-quit-20261020: red.log,
green.log, focused.log, focused-isolated.log, focused-serial.log, mac-build.log,
ios-build.log and build-artifacts.json. Swift used the existing scratch directory
/tmp/fort059-host-startup-tests and module cache /tmp/fort059-swift-module-cache.
The final filter is settingsSheet|agentSettings|modelPicker|avatarMotion|
providerNeutralCatalog|ownerSetupModel|ordinaryMacModel|ordinaryMacReinstall|
ordinaryMacRemovalComposition. Each build used CODE_SIGNING_ALLOWED=NO and a fresh
separate derived-data directory. Produced artifacts are:

- /tmp/fort059-settings-quit-native-build-20261020/Build/Products/Release/FortMac.app;
  native executable SHA256 4bd16858216b5c89dab930cd8f5ac342f422a0edf95681560492839143b6fd06.
- /tmp/fort059-settings-quit-ios-build-20261020/Build/Products/Release-iphonesimulator/FortiOS.app;
  executable SHA256 4c64e3ac859e68a2abcb0f1a282915e51753bf674fdf56767bb29da4f1c17443.

Neither artifact was signed, installed or launched. The installed empty-MCP report
0ab05d939bcbdc7eb1d0905156790a42a1db4375b5bb1ac63d6106cb95d563fc remains the active
handoff artifact. Its app, sign-in UI and active host/pairing were left untouched;
no codes or credentials were inspected or generated. Owner confirmation of personal
ChatGPT authentication is awaited; zero of six prompts have been sent. Account
selection, Astra discovery, final cleanup and changed-build installed Quit remain
pending. A normal client Quit is not host unregistration or credential removal.
G2 upload is separately held. Earlier stopped/unregistered statements in this file
are historical observations of their exact earlier builds and test phases.
