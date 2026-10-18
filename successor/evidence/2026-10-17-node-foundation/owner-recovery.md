# Durable owner stop and local pairing recovery

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md), under
[decision 059](../../../docs/architecture/decisions/059-node-foundation.md).
Source base `29f7050`; October 18, 2026. This is synthetic source/API evidence,
not installed lifecycle or provider acceptance.

An owner stop previously existed only in memory. A public API regression queued
a routine, stopped the host and reopened its ledger; dispatch returned `true`
and executed that routine. Ordinary queued plain-text work already had a restart
hold, so that failure must not be generalized to all queued messages.

Owner enrollment now records the latest device identity. Stop/disconnect records
its intent in SQLite before runtime disposal. Reopening keeps dispatch,
schedule catch-up, enrollment and new commands paused while preserving history
reads. Runtime readiness reports unavailable. Explicit local
`start({resumeOwnerHost:true})` resumes a stopped owner host; it cannot renew an
expired credential or clear a pending disconnect whose device still exists.
Temporary sessions cannot use this operation.

`recoverOwnerPairing({installationId,deviceId})` is a trusted composition API,
with no HTTP route. It checks the exact installation and latest owner pairing
before stopping anything. It waits for runtime disposal and then removes that
device and enrollment codes. A failed disposal keeps pairing and permits an
explicit retry. The recorded identity permits confirmation after credential
expiry or a lost disconnect reply; enrollment of a new device supersedes that
proof. Existing owner ledgers without this metadata can recover a still-present
exact device row, but cannot invent proof for an already absent unknown device.

Startup and recovery cannot overlap. A second failing regression showed recovery
could finish while an explicit start was awaiting its listener, allowing start
to erase the newly recorded intent. Recovery now rejects that overlap before
any mutation and can be retried when startup completes.

Passed:

- 14 focused owner pairing/recovery tests, including eight new public API tests:
  restart pause, explicit resume, schedule catch-up, expired/previously revoked
  pairing, preserved history, foreign/superseded identity, failure/retry,
  pending-disconnect protection, concurrent lifecycle calls, no recovery HTTP
  route, temporary-session rejection and no credential renewal.
- All 844 portable tests, zero failures/skips, including the compiled core's
  independent runtime contracts and private channel checks.
- Strict TypeScript checking and the full/implementation spec gates.

Logs: `/tmp/fort059-owner-recovery-red.log`,
`/tmp/fort059-owner-recovery-start-race-red.log`,
`/tmp/fort059-owner-recovery-controls.log`,
`/tmp/fort059-owner-recovery-portable.log`,
`/tmp/fort059-owner-recovery-typecheck.log`.
The first sandboxed regression invocation could not bind loopback (`EPERM`);
the approved temporary-listener run reproduced the behavioral failure above.

The returned proof covers runtime disposal and pairing revocation only. It does
not prove Node/helper process absence, startup unregistration, native credential
removal or application removal. The pinned Swift owner adapter must establish
those separately. Native builds/signed bridge checks were not repeated for this
core-only slice. No real provider, account, credential, startup registration,
installed app or historical demo was activated or changed.
