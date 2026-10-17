# Proposed native acceptance batch — October 17

Preparation only. The owner has not approved the three temporary enrollments.
No session root, credential, provider process or installed UI session was created.
Source: `996e19849f5104e71b26b7c14d201aaa56b31c87`.
The instruction-skill migration is already committed at
`a524c4a6fbe4d53e9f8e514a5e89a71f8cda8968` and included in these builds.

Run these commands from the worktree root. They inspect files and run only the
preparation checker's small unit suite:

```sh
/opt/homebrew/bin/python3 -B successor/ops/mvp-native-acceptance-20261017/test_check.py
/opt/homebrew/bin/python3 -B successor/ops/mvp-native-acceptance-20261017/check.py --check
```

A successful check reports `preparationChecks: passed` and `activation: blocked`.
It hashes the built apps and pinned runtime without executing them, verifies product
source and all six historical packets/native cache, and confirms proposed session
roots remain absent. A changed source/build requires new verification, not renewed
approval for unchanged scope. A changed scope requires reconciliation. There is
intentionally no `--start` or approval-writing action.

## Launch prerequisites

1. Record the owner's explicit answer to the existing proposal in
   `../mvp-native-test-scope-20261017.json`; do not infer it from development authority.
2. Implement and test a new launcher against that exact scope, then seal its source
   and app pins. `prototype/manual.ts` remains bound to a consumed historical packet;
   do not run it or weaken its entry gate. Current runtime recovery supports a bounded
   injected replacement factory; the historical launcher supplies no such factory.
3. The new launcher must enforce all three distinct roots, consume each attempt
   before any launch, admit at most 12 synthetic plain-text turns per session, and
   allow only one explicit service/runtime restart across both UI and terminal paths.
   At most three owned process launches share the original absolute deadline; each
   process has at most 300 seconds including cleanup. It must stop admission at
   540 seconds, finish cleanup before 600, and never extend or renew enrollment.
   Test late cancellation, launch/cleanup failure, a concurrent restart, process death,
   budget exhaustion, failure before READY, and same-ledger restart with fake children.
4. Preserve the native cache by an exact archive before pairing. Do not overwrite
   existing archives. Stop the exact app before archiving; never touch another app or
   credential. Fresh session roots are listed in the proposal and must not be reused.
5. Only the designated UI operator `01a1451b-81d7-76cf-8e5c-a85d7164201c`, coordinated
   through the parent, performs the attended sessions. Capture the exact installed
   build's accessibility tree and screenshots. Suppress enrollment values and tokens.

The current preparation is executable. The live batch is **not launch-ready** until
items 1–3 are satisfied. No fabricated launch command is supplied.

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

## Session 1 — memory, same Home and restart (up to six turns)

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

## Session 2 — native bundle lifecycle and historical revocation (up to four turns)

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
5. Open current B and verify it remains enabled. Invoke B for a new plain-text title
   request. Require B in the accepted configuration; no A invocation is authorized.
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
   for one short question about the saved departure. Verify the same shared memory
   revision and distinct exact recipient identities. Open both Homes and confirm the
   shared note is absent there.
5. Send one synthetic request and use Stop while it is active if timing permits.
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
