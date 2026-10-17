# Native session path recovery — October 17, 2026

Launcher source `409a11eaa7752d39b26ff33d89305648e6865b7d` fixes container discovery
and exact app identity. The verified cleanup reconciliation releases only the
remaining session gate. Session 1 stays consumed and blocked in its original file.
No app, provider process, enrollment, message or live session was created by this
repair. The owner approval and scope bytes are unchanged.

## What changed

The launcher checks the pinned app's bundle identifier and executable against its
Info.plist and the registered container's `MCMMetadataIdentifier`. Native storage is
inside `~/Library/Containers/io.fort.candidateb.mac/Data/Library/Application Support/FortCandidateB`.
The old non-container directory is no longer a default or fallback. The original
archive made there by session 1 is preserved.

NSWorkspace enumerates only the exact bundle identifier. Canonical app and executable
paths handle `/tmp` resolving to `/private/tmp`. A different app copy blocks the
operation. Termination uses NSRunningApplication for the exact matching app;
there is no broad process-name kill. The existing bounded native stop polling and
scoped credential cleanup remain in use.

`mvp-reconcile.ts` is a one-case recovery command pinned to session 1's original
state and retained-lock SHA-256 values. It verifies the unchanged approved scope,
expired original deadline, app/runtime absence, closed listener, empty actual pending
queue, absent connection/pairing metadata, zero enrollment/device rows and all three
completed run IDs. The Python reader uses SQLite `mode=ro&immutable=1`, requires an
already empty WAL, and checks file hashes, sizes and mtimes before/after. It never
starts the service, checkpoints the ledger or reads message text/token values.

The separately written `cleanup-reconciliation.json` records these checks and the
operator's earlier exact Keychain deletion result (status 44, already absent).
That Keychain result is attributed to operator turn
`01a14a81-8c2b-7271-b508-e739b6832403`, command
`exec-fd0cf4eb-034d-40a4-9f5f-62967177cc45`; this repair did not inspect Keychain.
The earlier check verified installation/device identity before deletion. There is
no claim of a new independent Keychain check or inferred device identifier.

After all checks pass, the original lock is moved to
`session-1/reconciled-active.lock`; its bytes and mtime are retained. The prior-session
gate requires the separate record, original state hash and archived lock hash.
Session 1 cannot restart. Session 2 becomes eligible; session 3 still requires
session 2 cleanup. Interrupted recording can resume without replacing evidence.

## Results and preservation

- 94 selected non-enrollment TypeScript tests passed; zero failed/skipped. This
  includes 26 new native/reconciliation tests and the existing 68 selected tests.
- Strict TypeScript checking passed. Three preparation-checker unit tests passed.
- Production native discovery and read-only reconciliation checks passed. Recording
  completed at the time in [cleanup-reconciliation.json](cleanup-reconciliation.json).
  [The launcher check](launcher-check.json) reports only session 2 currently available.
- [Preservation checks](preservation.json) confirm all 23 original batch, native-cache
  and six historical-packet files have the same bytes, size and mtime. The batch lock
  is included at its preserved new location. Main checkout status is unchanged.
- Existing native source remains `996e19849f5104e71b26b7c14d201aaa56b31c87`.
  App file pins match; the Mac app's strict signature verification passes in the
  permitted host context. A restricted-context signature check initially returned
  `CSSMERR_TP_NOT_TRUSTED`; no signing or trust settings were changed.
- Initial new-test runs failed because their interfaces were not implemented; one
  intermediate fixture run also exposed a `/var` symlink in the temporary test root.
  The fixture now resolves its root before exercising rejection behavior. Final
  checks are passing. A mistyped one-off Node read command failed before executing;
  the production immutable reader subsequently passed against the real ledger.

The cleanup was recorded using `0fd865d34e6a56d4f74a797d28e615b3f1fcf328`.
The final guard additionally requires the original state hash even if someone edits
its cleanup flag; a behavioral regression failed before that fix and passes now.
The state and reconciliation record themselves remain unchanged.

The old seal is preserved in [previous-pins.json](previous-pins.json). The new seal
contains 117 product files and the same app/scope/runtime pins. The scope SHA-256 is
`24232f4c96da97a3fa2b64620bedcc4752332d17e111bc8c7d583eac26bdf4f6`.
Owner approval remains the separate local record for the 15:32:58 UTC approval.

## Operator handoff and limits

From this worktree, the designated operator first runs:

```sh
/opt/homebrew/opt/node@26/bin/node successor/prototype/mvp-manual.ts --check
/opt/homebrew/opt/node@26/bin/node successor/prototype/mvp-manual.ts --session 2
```

Then follow [the existing checklist](../../ops/mvp-native-acceptance-20261017/PLAN.md)
and enter `start` once in the attended terminal. Session 3 follows only after
confirmed session 2 cleanup. No new approval is needed for these unused sessions
within the unchanged scope. There are at most two fresh enrollments and 24 further
accepted turns: at most 27 across the batch including session 1's three. Each
remaining session keeps its own 600-second total, 540-second work cutoff, one
explicit restart and three bounded runtime launches. Nothing carries over from
the failed session. Existing ChatGPT, exact Astra and no-tool controls are unchanged.

The new helper's actual termination/restart branch has fixture coverage but has not
been exercised against a running native app in this repair. Full enrollment-fixture
suites, native HTTP/Swift/build runs and installed/provider sessions were not rerun.
Session 1's installed memory/history evidence is retained; its failed restart and
remaining checklist are incomplete. Physical iPhone and broader MVP/release gates
remain open. The agent host retains the Mac user's access; the computer-tool profile
does not establish host isolation. No provider cleanup, push, merge or deployment
occurred.
