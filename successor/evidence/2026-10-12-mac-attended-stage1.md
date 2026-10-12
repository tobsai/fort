# Stage 1 native fixture verification — attended Mac pass, 2026-10-12

**The Stage 1 native fixture exit is verified.** Both native clients have passing
send/display/reconnect/restart evidence against the shared versioned fixture
contract. This records technical verification under the existing implementation
authority; it does not add provider, deployment or release authority.

This attended session completes the remaining current-app Mac journeys. The
owner authorized starting Stage 1 verification while present at the unlocked Mac.
The app was built from `986afd30f2967287764c5e10f11282a0e4f33a22` and remained
byte-identical throughout. Recovery and approval/cancellation used its prepared
runner. Uncertainty passed with a one-line test-selector correction at
`8cfc9ca1375ce880021455757c25afa129d80720`; no product code changed.

## Exact Mac results

| Journey | Passed evidence |
| --- | --- |
| Recovery | Normal enrollment, exact typed text/reply, app restart, cold offline history/pending, offline Send, reconnect, deliberately lost acceptance response, service/app restart and reconciliation. Five app launches; exactly three user messages, three replies and three completed runs; one dropped acceptance response. |
| Approval/cancellation | One approved turn produces its reply. A second turn is canceled while waiting for authority and produces no reply. Exactly two user messages, one reply, run states `completed,canceled`. |
| Uncertain outcome | An injected loss after execution start leaves `outcomeUnknown` and blocks the later accepted turn. Service/app restart preserves the barrier. Dismissing confirmation leaves it intact. Confirmation during a held refresh sends exactly one successful close command; original uncertainty remains recorded, only the later turn completes, and app restart retains both states. Two user messages, one reply, states `closedUnknown,completed`; no retry of the original request. |

Each passing journey reports **one passed test, zero failures and zero skips**.
The Node harness also passed its authenticated canonical-ledger assertions after
XCTest completed. It used fresh local fixtures and did not edit database rows.

Raw result bundles:

- `/tmp/fort058-mac-986afd3-owner-recovery-1.xcresult`
- `/tmp/fort058-mac-986afd3-owner-approval-1.xcresult`
- `/tmp/fort058-mac-986afd3-owner-unknown-2.xcresult`

The corresponding `.xcresult.log` and `-harness.log` files remain in `/tmp`.
[Result counts, log hashes, ledger assertions and artifact identity](native-2026-10-12/mac-attended-results.json)
are committed alongside this note. All six final fixture-window screenshots were
visually inspected: [conversation](native-2026-10-12/mac-attended-conversation.png),
[restart](native-2026-10-12/mac-attended-restarted.png),
[offline](native-2026-10-12/mac-attended-offline.png),
[lost acknowledgement](native-2026-10-12/mac-attended-lost-ack.png),
[approval/cancellation](native-2026-10-12/mac-attended-approval-cancel.png), and
[uncertainty closure](native-2026-10-12/mac-attended-unknown-closed.png).
The [enrolled accessibility hierarchy](native-2026-10-12/mac-attended-accessibility.txt)
and XCTest interactions verify named controls and exact transcript/status text.
This is the bounded native fixture inspection, not full VoiceOver/accessibility acceptance.

## One test failure, corrected without an app change

`/tmp/fort058-mac-986afd3-owner-unknown-1.xcresult` failed when the test asked for an
unscoped Cancel button. macOS exposed one in the confirmation sheet and another
in the Touch Bar. The app had correctly reached confirmation, and zero closure
commands had been sent. The [captured issue](native-2026-10-12/mac-attended-selector-failure.txt)
shows both matches. Scoping the query to `app.sheets.buttons["Cancel"]` fixed the
observed ambiguity. The corrected uncertainty test passed. No authorization
failure was retried, and no production behavior was changed to satisfy the test.

The original prepared products were copied before rebuilding the test runner to
`/tmp/fort058-mac-986afd3-original-products`. Every recorded original app, runner
and test-bundle hash matches that preserved copy. The active products remain at
`/tmp/fort058-mac-current-986afd3-build/Build/Products`; the app still matches all
four original hashes, while the runner/test bundle now include the test fix.
Both original and updated runner identities are accounted for by the prior build
record and current attended record. The existing development identity was reused;
strict/deep signature verification passes and entitlements are unchanged.

## Why this closes Stage 1 only

Prior [iPhone fixture evidence](2026-10-12-mac-and-contracts.md) covers messaging,
offline persistence, approval/cancellation and restart/lost-response recovery.
The [iPhone uncertainty and two-device race follow-up](2026-10-12-iphone-closure-and-race.md)
adds the current closure fix, two installed Simulator clients with one durable
approval winner and exact artifact checks. The combined evidence satisfies
Spec 058's **Stage 1 native fixture slice**. iPhone evidence is Simulator evidence;
no physical-iPhone result is implied.

The 46 service/runner tests, 19 Swift tests, strict TypeScript and independent
review are prior verified evidence for this unchanged application implementation;
they were not rerun during this attended UI session. The one-line test correction
was compiled by the successful Mac `build-for-testing`. Provider execution stays
disabled. The [T01–T20 map](2026-10-12-spec058-matrix.md) remains partial/unrun as
appropriate: enforced hostile-runner OS isolation, official Codex/Claude runtime
and authentication feasibility, physical iPhone/TLS, full accessibility, APNs,
backup/restore, skills/routines/scheduling, self-hosted topology and distribution
still have their separate gates. Stage 1 is not native MVP release acceptance.

## Session boundary and cleanup

Normal UI automation completed in this owner-attended session. The agent entered
no credentials, requested no additional permission, and changed no persistent
security, Automation Mode, Keychain, Gatekeeper, signing or entitlement setting.
The prior authentication failure and diagnosis remain historical evidence; this
successful run does not establish what caused that earlier failure.

All fixture app, UI runner, UI harness and worker processes exited. The temporary
credential-bearing run configuration is absent. The harness closed its listeners
and deleted its service ledgers. Ordinary app-local fixture test storage was not
cleared; distribution/uninstall residue checks remain a later gate. Older Mac and
both iPhone products retain all their previously recorded hashes. Unrelated
`reviews/` content is preserved. There was no provider login or cleanup, deployment,
push/publication, startup registration or external account connection.
