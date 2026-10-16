# Owner-trial timer repair and replacement — unstarted

Source: `46040913df5404359aaf9058b54e1133d4d69fa0`. [Authorization provenance](authorization.json) reuses the
preserved ten-minute/five-owner-message decision and records the coordinator's
replacement-preparation instruction. No live start occurs in this task.
[Operator handoff](../../ops/candidate-b-owner-trial-replacement-20261016/MANUAL.md).

The consumed owner trial failed before READY at `session/runtime-launch`: its
540,000 ms plan hit the owned launcher's 300,000 ms ceiling. The saved state has
one start, zero receipts and cleanup deleted; no port was recorded. UI reported
zero pairing/messages and no remaining runtime/app/listener/lock. Preparation
verified exact state/ledger file sets and hashes without opening SQLite. It did not
repeat a provider operation or read credentials. The initial test reproduced that
actual guard error before the repair; original red logs remain preserved.

The generic launcher still permits at most 300,000 ms. Only the exact
`owner-trial-replacement-20261016` packet with the literal work/total/reserve tuple
540,000/600,000/60,000 ms permits longer work. Unknown/consumed identities, modified
or extra budget fields and larger values are rejected before spawn. TERM/KILL grace
limits remain 2,000 ms each. At maximum work duration those use at most 4,000 ms of
the 60,000 ms cleanup reserve, leaving 56,000 ms for other cleanup. Controller and
plan use one declaration; independent literal limits in actual launcher validation
and integration tests catch drift. This lifetime scope is an application contract,
not a security identity or sandbox grant. Existing manual entry/pin checks remain.

478 focused tests passed; zero failed/cancelled/skipped. Strict TypeScript and
whitespace checks passed. [Validation](validation.json) distinguishes exact-plan
validation from the harmless Node-fixture launch. The actual Codex binary and native
app were never started; no pairing, provider message, sign-in or credential operation
occurred. Native app, permission/protocol code and service/contracts are unchanged.
[Exact pins](pins.json) cover 470 source files, the same five app files and
same Codex 0.157.1 binary. Existing same-user host and OS-enforcement limits remain.

[All five consumed packets](attempt-preservation.json) are preserved, including the
failed owner trial. [The fourth native cache archive](native-cache-preservation.json)
still matches every hash; the native cache path remains absent. The new replacement
root remains absent, with zero starts consumed. Main checkout status is unchanged.
The previous owner-trial packet and its historical pins are never reset or rewritten.

Only the designated UI task may execute on the coordinator's handoff. One start,
no restart, up to five human-entered messages in Writer Home, no skill, same existing
ChatGPT/Astra. Immediately report READY to the parent with exact work cutoff and
credential expiry; send no operator sample. Automatic shutdown and scoped cleanup
remain at nine/ten minutes. Preparation completion is not a working live-trial claim.
