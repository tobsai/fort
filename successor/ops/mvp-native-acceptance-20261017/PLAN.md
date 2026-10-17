# Completed native acceptance batch — October 17

The owner approved the unchanged three-session scope at 2026-10-17 15:32:58 UTC.
The separate local `owner-approval.json` records user message
`Sentinel_c9ad1471c1088191a9141d2ed2797c03`; the original proposal snapshot is retained.
Current launcher source: `e5a4ef54e67cfd2df506f42b191b8f80378581d8`.
Unchanged native artifact source: `996e19849f5104e71b26b7c14d201aaa56b31c87`.
Source/app/scope pins are in `pins.json`; its `activationAuthorized: false` means the
seal itself does not grant authority. The separate owner record supplies authority.

All three sessions are consumed. Sessions 1 and 2 retain their blocked records,
three and one completed turns respectively, and separate cleanup reconciliations.
Session 3 finished with six accepted turns, five completed and one canceled, two
launches, one restart and confirmed native/runtime/credential cleanup. The batch
total is ten accepted, nine completed, one canceled. Never replay a checklist or
transfer unused allowance. The commands below are historical execution records.
Read [the completed outcome and new offline preview](../../evidence/2026-10-17-mvp-demo-readiness/README.md).

The actual macOS bridge failure was an inconsistent path comparison: Foundation
returned `/tmp` while Node supplied `/private/tmp`. Both sides now use Foundation
normalization inside the helper; Node retains its filesystem check. The actual
bridge passed an inert AppKit canary, including graceful exit and wrong-copy refusal.
Read [shutdown recovery evidence](../../evidence/2026-10-17-mvp-native-shutdown-recovery/README.md).
The former `check.py --check` is a historical pre-activation checker that expects
absent session roots and the old native cache location. Use the current launcher
`--check` below for this resumed batch. The old checker and its three passing unit
tests are retained, not used to assert current live state.

The packet admits only the exact one-file Check marker bundle produced by the
instruction editor: CHECKED at version 1, then REVIEWED at version 2. The complete
canonical digest binds the entry bytes, file set, source/license/runtime metadata
and absent tool/script authority. Additional assets or metadata are refused. This
restriction applies to the temporary acceptance packet; general reviewed bundles
remain supported by the service and adapter.

## Historical activation gate and operator commands — do not execute

The corrected launcher is committed and sealed across 119 product files. All 99
selected non-enrollment tests, one actual AppKit canary and strict TypeScript
checking pass. The enrollment-fixture suite remains unrun under the existing execution
boundary; do not bypass that boundary. The operator subsequently observed supported live Fort termination and restart
in session 3. The recorded native source remains the completed batch artifact.

Keep `../mvp-native-test-scope-20261017.json` and the existing owner record unchanged.
Routine code and cleanup reconciliation do not expand enrollment, duration, message,
restart or privilege limits. This approval is now exhausted; it cannot authorize another session. Platform
permission prompts still require their actual disposition.

The operator first runs this read-only command from the worktree root:

```sh
/opt/homebrew/opt/node@26/bin/node successor/prototype/mvp-manual.ts --check
```

Historical command used by the designated operator for the now-consumed session:

```sh
/opt/homebrew/opt/node@26/bin/node successor/prototype/mvp-manual.ts --session 3
```

Enter `start` once. Wait for READY, then open the exact `appPath` and pair using the
local `pairingFile`; never print its contents or capture the code/token. Immediately
report the work cutoff and credential expiry to the parent. The terminal accepts
`status`, `native-status`, `restart`, `done` and `abort`; no prompt text is accepted
there. App messages must match the fixed synthetic samples below. This was the final
attended window; no turns, enrollment or restarts remain available.
No automatic loop, retry, renewal or fourth session is available.

Before terminal `restart` or `done`, first resolve any active/queued request, quit
the exact Fort app through supported UI, and enter `native-status`. Require **Native
app exit confirmed**. This read-only command consumes no launch or restart. If exit
is unconfirmed, stop and report it; do not spend a restart on a detection failure.
The same original deadlines and automatic bounded cleanup remain in force.

The single restart allowance is shared by terminal `restart` and the native
**Restart runtime** action. Terminal restart stops the app, confirms no active or
pending request, closes the old service/runtime and reopens the same ledger/port
with the original installation and credential deadline. Relaunch the same app
without enrollment. A runtime-only restart leaves the app connected and awaits
confirmed old-process cleanup before creating the replacement.

The controller enforces 12 accepted turns, one fresh enrollment, one explicit
restart and at most three owned launches per session. Each child gets at most
296 seconds of work plus four seconds for TERM/KILL; the original session deadline
is shared. Admission stops at 540 seconds and cleanup must finish before 600.
One durable consumed state and a batch lock prevent reuse and concurrent sessions.
Unconfirmed cleanup retains the lock and blocks later sessions. A missing/moved
ledger is never recreated as proof of cleanup. Unknown turns are never resubmitted.

Before pairing, the launcher validates `io.fort.candidateb.mac` against the app
Info.plist and registered container metadata. NSWorkspace compares both expected
and observed bundle/executable paths using Foundation normalization, then Node
verifies filesystem identity; another copy of that bundle blocks the operation. The storage
root is `~/Library/Containers/io.fort.candidateb.mac/Data/Library/Application Support/FortCandidateB`.
After confirming the exact app is stopped, it moves that cache into
`session-N/native-cache-before`, recording and verifying its hashes. It preserves earlier session histories and all six older
packets. Session 1 retains its original non-container archive. Session 2's archive preserves
the actual session 1 container history; session 3 preparation preserves session 2's
current container history in its own archive.
Do not reset roots, delete locks, replace archives or reuse the historical
`prototype/manual.ts`. No historical activation gate was modified.

Only UI operator `01a1451b-81d7-76cf-8e5c-a85d7164201c`, coordinated through the
parent, performs the attended sessions. Capture accessibility and screenshots of
these exact artifacts. Permission prompts require their actual disposition; never
change OS security/trust settings to bypass them.

## Common session contract

Use existing ChatGPT sign-in, official Codex 0.157.1, exact selected `gpt-6-astra`,
and the unchanged no-tool permission profile. No sign-in, fallback, API billing,
sensitive data, tools, startup services or public listener. The agent host retains
the Mac user's access; the computer-tool profile does not establish host confinement.

Before each action verify time remains before the work cutoff. Record only source,
app and runtime identifiers; request/run/receipt IDs; state transitions; counts; and
redacted cleanup results. Effective model stays unreported unless the provider proves
it. Failures consume a start. Stop when identity, cleanup or outcome is uncertain;
do not spend another session automatically or retry an unknown turn.

Create Writer with “Reply briefly using supplied context.” and Reviewer with
“Check the sample claim.”, both using the exact selected model. These are synthetic
Agent instructions. No repository or account content belongs in any prompt.

## Session 1 — consumed; historical checklist only

Steps 1–4 passed with three completed receipts. The restart at step 5 failed.
Steps 5–7 remain incomplete and are not authorized for replay in this packet.

1. Pair the fresh Mac installation. Verify Writer Home, selected model and host.
   Open Memory, save `The sample packing item is a red notebook.` and reload it.
2. Send `What sample packing item is saved in this conversation's memory?`.
   Require one receipt, one terminal reply and the exact memory revision on the run.
3. Send `Our sample code word is cedar. Reply with that word only.` Wait for terminal
   evidence. Relaunch only the app, reconnect without enrollment, and verify history.
4. Send `What sample code word did I give you earlier?` Require the same Home and
   retained history, one new receipt, no duplicate prior request.
5. Invoke the one explicit service/runtime restart through the new launcher, after
   confirming no active request or unresolved effect. Reuse the same ledger, port,
   installation and credential deadline. Reopen the app without reenrollment.
6. Send `What sample packing item and code word have we used?` Require the current
   memory revision and same-Home history. Clear Memory, refresh, and verify future
   configuration has empty memory while accepted historical turns retain their notes.
7. Open Reviewer Home; verify Writer memory is absent. Send `Reply with the single
   word ready.` and verify exact Reviewer attribution. Do not claim model behavior
   alone proves isolation; inspect the recorded configuration and context scope.

## Session 2 — consumed; historical checklist only

A invocation, inert routine pinning, B review/enable and historical A revocation
passed. Restart failed before a replacement runtime launch. B invocation/export
and all routine execution remain untested; do not replay this consumed packet.

1. Create an instruction skill named Check marker with `End with [CHECKED].` through
   the native editor. Verify it produces a complete disabled bundle. Inspect source,
   license, files, digest and requested authority; explicitly enable revision A.
2. Select A for Writer Home and send `Suggest one short title for a sample packing
   list.` Require the exact selected revision in the receipt/run and a terminal reply.
3. Save a routine named Marker review, pinned to Writer and A, containing one simple
   task. Saving executes nothing; do not run it or enable a schedule in this batch.
4. Edit the skill into B with `End with [REVIEWED].` Verify B starts disabled and A
   remains the routine pin. Review and enable B. From Routines choose “Review pinned
   skill”; verify exact A identity/digest, disable A, and reload to confirm A is revoked.
5. Open current B and verify it remains enabled. Invoke B with `Suggest another short title for a sample packing list.`. Require B in the accepted configuration; no A invocation is authorized.
6. Export the inert bundle to the session's synthetic evidence directory and inspect
   the export using the native review workflow. Importing or previewing enables nothing.

Queued/new routine rejection after A revocation is already a synthetic HTTP regression.
Actual-provider routine/schedule execution is outside this plain-text batch proposal.

## Session 3 — shared context, explicit recipients and cancellation (up to six turns)

1. Create the existing experimental shared conversation with Writer and Reviewer;
   this exercises manual recipient selection, not automatic agent delegation.
2. Select Writer and send `Our sample plan is to pack a red notebook. Repeat the
   item.` Require Writer attribution and one receipt.
3. Select Reviewer and send `Check that sample plan and name the packing item.`
   Require Reviewer attribution and only this shared conversation's history.
4. Save shared Memory `The shared sample departure is Monday.` Select each recipient
   with `What departure is saved in this shared conversation?`. Verify the same shared memory
   revision and distinct exact recipient identities. Open both Homes and confirm the
   shared note is absent there.
5. Send `Write a short synthetic packing checklist for a red notebook.` and use
   Stop while it is active if timing permits.
   Record the real observed outcome: completion, confirmed cancellation, or unknown.
   Do not invent cancellation evidence if completion wins. For unknown, inspect and
   explicitly close the old outcome; never resubmit that request.
6. If readiness reports restart required, use the one explicit bounded restart only
   when cleanup is confirmed and its budget remains. Send one **new** `Reply with the
   single word ready.` request in the same conversation. The old request remains
   terminal/unknown-closed with its original receipt; it must not run again.

Setup cancellation, unavailable-model recovery, transport loss and cleanup refusal
are covered by synthetic actual-adapter regressions. Do not change models or disable
runtime controls to force those failures in the live batch.

## Mandatory end of each session

Stop admission, stop the exact app, interrupt/reap owned runtime groups, stop the
listener, revoke only this session's enrollment/device rows, remove only its issued
Fort Keychain entries and temporary pairing files, and inspect absence before expiry.
Retain the ledger, state and synthetic cache archive. A failure or unconfirmed cleanup
is a blocker, not an invitation to reset state or renew credentials. Record actual
turns, launches and restarts against all limits, and distinguish build/HTTP results
from installed accessibility, screenshots and correlated provider receipts.

Physical iPhone, reachable authenticated service, cross-device read state, real host
sleep/reboot, notifications, backup/restore, complete host isolation and distribution
remain separate acceptance gates.
