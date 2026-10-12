# Installed iPhone uncertainty closure and approval race — 2026-10-12

Branch: `codex/fort-core-openmaus-20261011` at `/tmp/fort-core-openmaus-20261011`.
Native model fix: `d044da90716cfc619f0f156b4a301a056bea8a72`. Installed journey and harness source: `b99d89ee43eba5abdd6f54ced0b876d0acff2391`.
These are fixture acceptance subsets under approved Spec 058, not full Stage 1
or T01–T20 acceptance. The quarantined Apache core donor remains based on
`8b53c0323dba2099a953ee2d77ab9330cd5acaa7`; this work changes no donor files.

## Confirmed closure during refresh

The installed iPhone journey creates an uncertain run through the existing
`afterStarted` fixture failpoint, queues another turn, and restarts both service
and app. The uncertainty barrier stays intact. Dismissing the confirmation leaves
it intact too. The test then holds a real `/v1/installation` read, confirms
**Close without retry** while that read is pending, and releases the read.
Exactly one close command receives HTTP 200. The original run becomes
`closedUnknown`, preserving uncertainty, and only the later request receives a
fixture reply. App restart retains both states. The harness checks canonical
message and run counts through the authenticated API; it does not edit database rows.

A real native bug was exposed: `closeUnknown` silently returned when background
refresh marked the model busy. `d044da9` waits for that refresh to complete and
then rechecks compatibility and the existing command guards. A public native HTTP
regression first failed because no close request was sent. It now passes for a
compatible installation and proves that a newly discovered incompatible contract
still sends no closure command. The change is shared FortKit code; the updated
Mac app has not been built or exercised yet.

## Two installed devices

Two separately enrolled iPhone 18 Pro / iOS 27 Simulators use the same fresh
fixture installation. Device A taps Approve; device B taps Reject. The fixture
proxy holds both complete native requests until both arrive, then forwards them
together. It proves distinct device credentials without writing their values.
Both commands address the same action, attempt, binding, digest and scope.

The observed winner was **rejected**. Both API responses contain the same winner;
the ledger has one decision event, one user message, no assistant reply and one
canceled run. Both installed clients show that result after app restart. Closing
and reopening the actual service preserves the identical snapshot. This single
installed race covers the rejection-winning outcome; an approval-winning installed
race was not separately forced. Existing HTTP races and installed single-device
approval evidence remain separate evidence.

## Exact validation

| Check | Result |
| --- | --- |
| FortKit public HTTP regression before fix | Failed as intended: confirmed close was discarded during refresh. |
| Full FortKit suite after fix | **19 passed**, zero failures; `close-refresh-green.log`. |
| Strict TypeScript 6.0.3 check | Passed with `skipLibCheck: false`. |
| iPhone Simulator build-for-testing | Passed; `/tmp/fort058-ios-acceptance-build6.log`. |
| Installed uncertainty + held-refresh confirmation | **1 passed**, zero skipped: `/tmp/fort058-ios-unknown-6.xcresult`; harness ledger checks passed. |
| Installed race device A | **1 passed**, zero skipped: `/tmp/fort058-ios-race-1-A.xcresult`. |
| Installed race device B | **1 passed**, zero skipped: `/tmp/fort058-ios-race-1-B.xcresult`. |
| Race ledger and service restart assertions | Passed; `iphone-race-ledger.json`. |
| Installed app hashes | Both devices match the build for executable, debug dylib, Info.plist and CodeResources. |
| Code signatures | Strict/deep verification passed for both installed apps, built app and UI test runner; local Simulator ad-hoc signing. |
| Accessibility and screenshots | Exact status/actions exercised through XCTest; all three final fixture screenshots visually inspected. |
| Preserved products | Previous Mac product's four recorded hashes and previous iPhone product's three recorded hashes still match. |
| Git whitespace check | Passed. |

The service/runner implementation did not change in this follow-up. Its previously
verified 46 Node tests were not rerun; this round's new execution covers the native
suite and installed fixture harnesses. Swift 6.4, Xcode 27 (27A266a), Node 26.5.0
and the already installed project TypeScript were used. No dependencies were added.

The evidence files beside this note include raw XCTest summary counts for all
attempts, artifact hashes, the red/green Swift logs and the race ledger summary:
[artifact hashes](native-2026-10-12/iphone-acceptance-build-hashes.json),
[XCTest summaries](native-2026-10-12/iphone-acceptance-xcresults.json),
[ledger summary](native-2026-10-12/iphone-race-ledger.json),
[uncertain closure screenshot](native-2026-10-12/iphone-unknown-closed.png),
[device A](native-2026-10-12/iphone-race-device-a.png),
[device B](native-2026-10-12/iphone-race-device-b.png).

## Failed attempts retained

- `unknown-1` assumed an explicit Cancel button. iOS 27 presented a popover with
  an accessibility dismissal region; the test was corrected to use that region.
- `unknown-2` did not close the run after confirmation. No request diagnostics
  existed for that attempt, so its precise timing cause is not claimed.
- `unknown-3` passed the original closure journey, motivating a deterministic
  overlap test instead of accepting one successful attempt as reliability proof.
- `unknown-4` failed the expected assertion that confirmation was disabled while
  a read was held. `unknown-5` showed that a view-level `.disabled` guard did not
  update the already-open SwiftUI confirmation. That attempted fix was reverted.
- The separate Swift red test reproduced the model's dropped command. The model
  fix and `unknown-6` establish the passing wait-and-recheck behavior above.

## Cleanup and remaining gates

Temporary Simulator `B9A5B0F7-3ED9-42DB-8335-F1AAD3186FB5` was removed after evidence
capture. Existing Simulator `E3C585CB-E212-4F42-ACA6-FABD14F7D1B9` was returned to
Shutdown. Fixture ledgers, one-use codes, run configurations and processes are
ephemeral and removed by the harness. Signed products and raw result bundles in
`/tmp` are retained for independent inspection. The prior signed products remain
at `/tmp/fort058-ios-build` and `/tmp/fort058-mac-trusted-build`; this follow-up
uses `/tmp/fort058-ios-acceptance-build`.

Mac UI tests remain paused for the owner's local authentication, as documented in
[the unchanged diagnosis](2026-10-12-mac-authorization-diagnosis.md). No Mac UI
relaunch, automation/security setting, signing identity or entitlement changed.
A later Mac run must build the current FortKit source in a separate build directory
before claiming current-tip installed acceptance; the preserved older Mac product
is evidence of its own revision only. Parent review of these new commits is still
pending. Physical iPhone, iPhone-plus-Mac race, complete accessibility, real
providers, enforced OS sandbox, TLS, release and all other open matrix cells remain
unrun. There was no provider login, provider cleanup, deployment, push/publication,
startup registration or external account connection.
