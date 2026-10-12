# Mac development signing and contract follow-up — 2026-10-12

**Later scoped diagnosis:** [Mac XCTest authentication](2026-10-12-mac-authorization-diagnosis.md)
records the unchanged-artifact retry and the host's explicit user-authentication
requirement. This record preserves the preceding passes and failures.

Branch `codex/fort-core-openmaus-20261011`, isolated worktree
`/tmp/fort-core-openmaus-20261011`. The donor remains unchanged at upstream
`8b53c0323dba2099a953ee2d77ab9330cd5acaa7`. No remote push, visibility change,
deployment or provider connection was made.

## Fixed review findings

The independent review of `ee150e0` confirmed all four earlier P2 fixes and
reproduced two more. Commit `c48e970` fixes both:

- A committed expired approval now schedules dispatch before returning the
  `approvalExpired` response. The regression uses real HTTP with `autoRun:true`;
  a second turn must leave `queued` without another command. Its red run observed
  `[canceled, queued]`; the green run observes `[canceled, waitingForApproval]`.
  Expiry still authorizes no reply for the first turn.
- An incompatible native contract now disables approval, cancellation,
  close-unknown and read acknowledgement as well as Send. The model and buttons
  share `supportsCommands`. The test advertises version `2.0`, preserves readable
  history and draft, and observes zero owner-control requests. Its red run emitted
  mutations and incorrectly reported a normal connection.

Four additional negative-contract tests exercise additive unknown fields,
unknown run states, unsupported version/runtime combinations, malformed snapshot
ordering/identity/cursors and incompatible required wire types. Valid projections
remain intact. These are fixture decoding/model checks, not old installed binary
upgrade proof.

Verified: **46/46 Node**, **18/18 Swift**, strict TypeScript with no suppression,
and `git diff --check`. Saved logs: [expiry red](native-2026-10-12/expiry-red.log),
[incompatible-contract red](native-2026-10-12/incompatible-red.log),
[Node green](native-2026-10-12/followup-node-green.log) and
[Swift green](native-2026-10-12/followup-swift-green.log).

## Existing development identity

The coordinating request authorized inspection of public signing metadata and
use of exactly one existing matching development identity for local debug tests,
provided no new credential, provisioning, Keychain permission or account change
was required. `security find-identity -v -p codesigning` returned exactly one
Apple Development identity, alongside distribution identities which were not used.
An older expired certificate had the same common name; the exact valid identity
fingerprint was matched before signing.

| Public metadata | Observed value |
| --- | --- |
| Certificate class/name | Apple Development: Tobias Gunn (76KFKR48Y3) |
| SHA-1 identity fingerprint | `58F40D1AB480565B89109B853235AC3A7B2E7AD1` |
| Team | `T3JB5MYZ93` — Maple Tree Enterprises LLC |
| Chain | Apple Worldwide Developer Relations Certification Authority → Apple Root CA |
| Validity | September 29, 2026–September 29, 2027 |
| Mac app | `io.fort.spec058.fixture.mac` |
| Build | Xcode 27.0 (27A266a), arm64 macOS, Debug |

`xcodebuild build-for-testing` used manual signing with that exact identity/team,
`CODE_SIGNING_ALLOWED=YES`, `CODE_SIGNING_REQUIRED=YES`, and empty provisioning
profile settings. No `-allowProvisioningUpdates` was supplied. Both app and test
runner passed host `codesign --verify --deep --strict`. The signed products are
under `/tmp/fort058-mac-trusted-build/Build/Products/Debug/`.
Per-build overrides leave project signing defaults unchanged.

No private key or credential value was exported or copied. Normal signing used
the existing identity without a new prompt. No account/certificate/profile was
created or downloaded; no Keychain permission, trust setting, Gatekeeper policy,
quarantine attribute or startup setting was changed. The earlier ad-hoc runner's
Gatekeeper rejection remains a valid failed attempt; the development-signed runner
subsequently launched through the normal system path.

## Installed Mac fixture inspection

The first trusted runs enrolled normally, stored the synthetic device credential
in Keychain, sent text and displayed the saved reply, but failed the transcript
accessibility assertion. A value-aware query in `ab2cb58` also failed. App-window
inspection then showed the actual defect: selectable message bodies were visible
but absent from the combined accessibility row, which exposed only the speaker.
Commit `e47730c` makes the message row explicitly expose speaker and exact message text. The same
installed journey is the regression; its original failure is retained at
`/tmp/fort058-mac-trusted-recovery-3.xcresult`.

Mac screenshot attachments now use the fixture window, and explicit hierarchy
attachments use that window's subtree. Whole-desktop recordings, system menus and
unrelated window diagnostics from Xcode's automatic result bundle are not copied
into repository evidence. Only inspected fixture-window artifacts are retained.

The full installed Mac recovery journey **passed** at
`/tmp/fort058-mac-trusted-recovery-4.xcresult`: exact text, saved reply, app restart,
cold offline cache/pending, offline Send, a lost acceptance response, service/app
restart and reconciliation. The harness verified one dropped response and exactly
three user messages/replies/completed runs. This run includes the accessibility
fix, before the subsequent conversation-pane identifier/test-scroll change.

On the final `e47730c` app, the separate approval/cancellation journey **passed**:
`/tmp/fort058-mac-trusted-approval-2.xcresult`. The harness verified two requests,
one reply and `completed,canceled`. The first attempt failed because the approval
button was below the visible pane; the corrected test scrolls the explicitly
identified Conversation pane before activating the actual button.

A recovery rerun on that final app **failed during app relaunch** with
`Not authorized for performing UI testing actions`, after the initial send/reply,
first restart and offline Send. Result/log:
`/tmp/fort058-mac-trusted-recovery-5.xcresult.log`. No permission change or further
manual Mac UI retry was attempted. Xcode automatically restarted its test runner;
the result summary records the original AX failure and a runner initialization
timeout while enabling automation mode. The failed test process and runner exited before targeted cleanup was needed;
the harness then closed its temporary listeners.
The earlier full recovery pass is not presented as an exact-final-binary pass.
The final app and runner still pass signature verification. Mac installed
acceptance remains open pending this host UI-testing authorization
and a complete final-artifact rerun. This was a runtime authorization error, not
an automatic tool approval rejection. A read-only Fort-process-filtered system-log
check showed the test session initially receiving automation mode and later
closing after failure; it did not identify a TCC denial or establish the underlying
cause. No specific permission prompt or required owner intervention is inferred.

XCTest summaries also contain framework runtime warnings about main-thread calls
and priority inversion; these were not test failures and are not treated as proof
of complete accessibility/performance acceptance. Physical devices and VoiceOver
remain untested.

Inspected fixture-window screenshots: [conversation](native-2026-10-12/mac-conversation.png),
[restart](native-2026-10-12/mac-restarted-conversation.png),
[offline pending](native-2026-10-12/mac-offline-pending.png),
[recovered acknowledgement](native-2026-10-12/mac-lost-ack-recovered.png), and
[approved/canceled](native-2026-10-12/mac-approval-cancel.png). The first four are
from recovery-4; the last is from approval-2. [Final Mac artifact hashes](native-2026-10-12/mac-build-hashes.json)
identify the directly launched build product used by approval-2 and the incomplete
recovery-5. They do not retroactively identify the earlier recovery-4 build.

## Final iPhone Simulator check

The shared `e47730c` changes compile with project-default Simulator ad-hoc signing.
The installed approval/cancellation journey passed at
`/tmp/fort058-ios-approval-followup-1.xcresult`, including canonical ledger checks.
Its [screenshot](native-2026-10-12/iphone-approval-cancel-followup.png) was inspected.
The complete final recovery journey also passed at
`/tmp/fort058-ios-recovery-followup-1.xcresult`: one dropped response, three exact
user messages, three replies and three completed runs. Inspected screenshots:
[conversation](native-2026-10-12/iphone-conversation-followup.png),
[restart](native-2026-10-12/iphone-restarted-conversation-followup.png),
[offline pending](native-2026-10-12/iphone-offline-pending-followup.png),
[recovered acknowledgement](native-2026-10-12/iphone-lost-ack-recovered-followup.png).
The installed executable, debug dylib and Info.plist all match the tested build:
[hashes](native-2026-10-12/installed-build-followup-hashes.json). XCTest terminated
the app and shut down its Simulator. The subsequent `simctl get_app_container`
lookup refused the shutdown state; the exact selected Simulator's on-disk bundle
was read and its bundle identifier checked without rebooting it.

The [extracted XCTest summaries](native-2026-10-12/xcresult-followup-summaries.json)
record one passed/zero failed for each successful journey and preserve the failed
Mac session separately. A final scoped process check found no remaining fixture
app or test runner.

## Remaining gates

See the [T01–T20 acceptance map](2026-10-12-spec058-matrix.md). The signed Mac
development fixture does not prove notarized distribution, a physical iPhone,
TLS placement, official provider traffic or hostile-runner OS isolation. Installed
unknown-outcome closure and two-device approval/revocation races remain unrun.
Full accessibility, restore/export/delete, skills/routines, schedules, APNs and
release lifecycle remain open. Neither provider is enabled; no new authority for
provider login, external accounts or cleanup is inferred from these results.
