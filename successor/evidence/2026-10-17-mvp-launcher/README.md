# Sealed three-session launcher — October 17, 2026

Candidate source: `a8a14c8d623e12af37b020735588e08bb4100180`, branch
`codex/fort-openai-rebuild-20261016`. Runtime stop/restart race fix:
`99ab5bc449c71f39d6e0fb835b8ac15166ff5014`.
Unchanged native artifact source: `996e19849f5104e71b26b7c14d201aaa56b31c87`.
The [operator plan](../../ops/mvp-native-acceptance-20261017/PLAN.md) and
[pins](../../ops/mvp-native-acceptance-20261017/pins.json) bind 108 source files,
the exact signed Mac app, the immutable scope snapshot and six historical packets.

## Result

A dedicated attended launcher replaces the previously missing preparation. It
cannot activate without a separate record of the owner's explicit approval of the
unchanged proposal. That file is absent. The read-only CLI and readiness checker
pass and report approval pending. No real session root, enrollment, provider process,
native cache archive, installed app or Keychain operation was created by this work.
The old manual and its six consumed sessions remain unchanged.

The controller enforces three distinct consumed starts, one enrollment per session,
12 accepted synthetic plain-text turns, one shared UI/service restart allowance,
and at most three owned process launches. Each child has at most 296 seconds of
work and four seconds for TERM/KILL. All launches share the original 540-second
work cutoff and 600-second credential expiry; restart never extends either.
Message replay retains the original receipt and does not spend another turn.
Fixed reviewed samples, exact selected model, Agent instructions and bounded note/
skill configurations limit admission. Routine/schedule execution is refused here.

Service restart waits for confirmed native/process shutdown and an empty pending
queue, then reuses the original ledger, port, installation and credential deadline.
Runtime-only replacement shares the same restart budget. Failed/unknown prior turns
are preserved rather than retried. Cleanup verifies the exact ledger identity and
deadline, revokes its enrollment/device rows, confirms zero remaining rows, erases
only issued native credential references, and removes its pairing file. Unconfirmed
cleanup retains the batch lock and blocks subsequent sessions. Historical native
cache bytes are archived and verified only during a later approved activation.

The independent review's P1 race is fixed: adapter disposal cancels and joins
replacement, rechecks shutdown before and after creating a connection, and waits
for any raced connection cleanup. Tests stop during old cleanup and new preflight.
A constructor refusal after child creation also returns a failed startup whose
cleanup is joined, so packet completion cannot leave that child running.

## Validation and exact limits

- **689 TypeScript tests passed**, zero failed/skipped, including **44 actual-adapter
  tests** and **22 dedicated launcher/preparation tests**. The launcher tests use
  temporary ledgers, synthetic credentials, cache fixtures and owned Node fixture
  children; they never run the official Codex executable or real credential store.
- Strict TypeScript check passed. One invocation initially used the repository root
  instead of the successor directory; the corrected invocation passed.
- Controller coverage includes 12/13-turn admission, duplicate replay, fixed absolute
  deadlines and reap reserve, same-credential service restart, concurrent restart,
  shared HTTP/service budget, shutdown during restart, failed preflight/launch,
  expired restart, native/runtime/credential cleanup failure, late cleanup, moved
  ledger identity, stalled HTTP input, constructor refusal and exactly three sessions.
- Approval gate, immutable cache archive and symlink/overwrite/live-app refusal tests
  passed. Read-only checker unit suite: **3 passed**. CLI and checker both passed
  current file checks and reported blocked activation.
- Existing native artifacts were reused because `successor/apple` is byte-identical
  to `996e198`. Their **36 Swift tests**, ten-execution/six-restart Swift HTTP journey
  and Mac/Simulator build-for-testing results are prior evidence, not rerun claims.
  Four signatures and artifact bytes are reverified in `preservation-and-builds.json`.
- Main checkout status, six historical packet files and native cache remain unchanged.
  Three proposed session roots and the owner-approval file remain absent. Preexisting
  untracked `.build/` is preserved. No dependencies, runtime permissions or accounts
  were changed. Logs and hashes are in `checks.json`.

The shutdown-race reproduction failed before the fix. The moved-ledger and post-spawn
constructor regressions also failed before their cleanup fixes. Red logs are retained.
An early controller fixture used macOS's symlinked temporary path and was changed to
its canonical path. No final check remains failing. A read-only preservation check is
file evidence; it does not read historical message contents or credential stores.

## Ready-to-test scope and remaining gates

No further blocking code defect is known for the bounded **Mac plain-text checklist**
after these checks. The candidate is prepared and sealed; it is not user-accepted.
The new launcher itself has not yet had an independent source review. The parent
reviewer independently verified the earlier scheduler, historical skill and runtime
continuation fixes, and reported the shutdown race fixed here.

**Permission gate:** the existing explicit owner decision for the three-session
proposal is still pending. Record actual approval separately; do not alter the
proposal or infer authority from these tests. Actual host permission prompts, if
encountered, must be resolved without security/trust changes or broader account access.

**Acceptance work:** inspect the exact installed Mac build using accessibility and
screenshots, then correlate real provider receipts for memory, history/relaunch,
restart, cancellation, bundle review/revocation and manual shared recipients. The
selected model is exact `gpt-6-astra`; effective model remains unreported unless the
provider proves it. The agent host retains the Mac user's access; the tool profile
is not OS-enforced host confinement.

**Broader MVP implementation and device gates remain:** the service currently binds
only loopback, so physical iPhone requires a separately prepared reachable authenticated
HTTPS endpoint and its authority, followed by two-device history/read-state tests.
Long-lived host/lifecycle management, notification handling, encrypted backup/restore,
export/deletion, Claude/self-hosted provider completion, complete tool/script skill
execution under enforced policy, upgrade/uninstall and distribution are not completed
by this temporary launcher. Real host sleep/reboot/schedule behavior also remains
unverified. No provider cleanup, startup service, ingress, deployment, push or merge
was performed or authorized here.
