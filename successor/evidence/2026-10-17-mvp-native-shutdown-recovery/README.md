# Session 2 native shutdown recovery — October 17

Source `e5a4ef54e67cfd2df506f42b191b8f80378581d8` fixes the actual macOS bridge
failure and adds a reconciliation limited to consumed session 2. The candidate is
sealed across 119 product files; native Fort artifacts remain unchanged at
`996e19849f5104e71b26b7c14d201aaa56b31c87`. Only session 3 is now available under
the existing owner approval. No live Fort app, enrollment or provider was started
by this repair. The canary runs below used a separate inert AppKit app.

## Root cause and tested fix

Node `realpathSync` returns `/private/tmp/...` for the pinned app. Foundation's
`URLByResolvingSymlinksInPath.path` returns `/tmp/...` for that same path. The
previous JXA helper normalized only the observed path with Foundation, then
compared it directly with the Node-normalized expected string. Every matching live
app at that location was rejected as a different app during the first status check.
`NativeSession.stop()` caught the error and returned false, so the helper never
requested termination. That explains one consumed restart, no second runtime
launch and the still-running Fort app.

The initial tests replaced the macOS bridge and did not exercise this difference.
The new regression compiles a minimal AppKit app with a unique test bundle identity,
no Fort data, no service or network, and a 15-second safety exit. It launches only
that owned process and calls the production JXA script. The old script failed twice
at the status check; targeted diagnostics showed `/private/var/...` versus `/var/...`.
[The pinned Fort path check](fort-path-normalization.json) confirms the corresponding
`/private/tmp/...` versus `/tmp/...` mismatch without launching Fort.

The fix applies Foundation normalization to both expected and observed paths before
comparison. Node still verifies their filesystem identity afterward. The final
canary uses `/private/tmp`, proves exact PID/path recognition, rejects mismatched
bundle and executable paths without terminating the process, then observes graceful
exit with code zero within three seconds and an idempotent second stop. It passes
against the actual macOS bridge. Temporary diagnostic code was removed; the red
capture is preserved in [native-bridge-red.txt](native-bridge-red.txt).

## Session 2 evidence and reconciliation

The original state remains `blocked`, launches 1, restarts 1, one completed receipt,
credential cleanup `deleted` and native cleanup `unconfirmed`. Its SHA-256 remains
`712a53c78a9b16dd643793ae88812e57a7485a015e6cd5db265f540e8fabfc3b`.
The completed run is `710cb653-25d6-4ca5-9ff5-fe4e13f6558f`, selecting skill A revision
`580f04b2-55b9-4b07-8b87-2587d71e2f33`. The operator verified A invocation, inert
routine pinning, B disabled/reviewed/enabled, exact historical A review and persisted
A revocation. B invocation/export and every live routine execution remain untested.

Fresh checks confirmed the actual app and official runtime absent, listener 55280
closed, pending queue empty, connection/pairing metadata absent, zero enrollment/
device rows, unchanged expired credential deadline and the one completed run.
The original ledger was read with SQLite `mode=ro&immutable=1`; no checkpoint,
credential mutation, provider action or transcript replay occurred.

The [separate cleanup record](cleanup-reconciliation.json) binds the original state
and lock hashes. The original lock is preserved at `session-2/reconciled-active.lock`.
All 33 preexisting batch/native/historical files retain bytes, size and mtime, and
the main checkout status is unchanged. Session 1's state, reconciliation and lock
are intact. Session 3's directory is still absent. See [preservation.json](preservation.json).

Keychain cleanup is explicitly attributed to the bounded controller's saved
`deleted` result and operator turn `01a14aa5-a8ff-70af-8478-2b279d766afc`, which
confirmed connection/pairing absence. This repair did not query Keychain, infer a
device identifier or claim a new independent Keychain inspection. That evidence is
distinct from session 1's earlier status-44 observation.

## Validation and operator commands

99 selected non-enrollment tests, one actual AppKit canary and strict TypeScript
checking passed. The production cleanup check, reconciliation and launcher check
passed. All four prior native artifact file sets/hashes match. No fresh Fort build,
Swift/native HTTP suite, signature check or full enrollment-fixture suite was run.
Detailed commands/results and log hashes are in [checks.json](checks.json).

Only the designated operator runs the remaining attended session, from this worktree:

```sh
/opt/homebrew/opt/node@26/bin/node successor/prototype/mvp-manual.ts --check
/opt/homebrew/opt/node@26/bin/node successor/prototype/mvp-manual.ts --session 3
```

Enter `start` once and follow session 3 in [the plan](../../ops/mvp-native-acceptance-20261017/PLAN.md).
Before terminal `restart` or `done`, quit the exact Fort app through supported UI,
then enter `native-status`. Continue only after **Native app exit confirmed**.
The status command consumes no restart or launch. If exit is unconfirmed, stop and
report it; do not spend the remaining restart on a detection failure. The corrected
automatic stop remains bounded and covered by the real canary, but live Fort
termination with this revision is not claimed as verified.

Session 3 retains one new expiring enrollment, at most 12 accepted turns, 600 seconds
total with the same 540-second work cutoff, one explicit restart shared across UI/
terminal, and three bounded runtime launches. Four turns were completed across the
first two sessions; the batch can now total at most 16. No unused allowance transfers,
session replay, additional session, provider/model change or privilege expansion is
authorized. Existing approval/scope bytes remain unchanged. Platform prompts still
need their actual disposition; no trust or security setting was changed.

No provider cleanup, push, merge or deployment occurred. The agent host retains the
Mac user's access. Physical iPhone, broader MVP acceptance and distribution remain
open.
