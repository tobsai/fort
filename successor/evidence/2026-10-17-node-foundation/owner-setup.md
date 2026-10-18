# Resumable local owner setup

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), under the existing
[Node implementation decision](../../../docs/architecture/decisions/059-node-foundation.md).
Verified October 18, 2026 on the existing isolated worktree. Synthetic credentials
and temporary ledgers only; no provider, Keychain, startup registration or ordinary
owner activation was used.

## Behavior

`beginOwnerSetup` returns a durable five-minute offer or the existing active
pairing. `completeOwnerSetup` accepts the client's previously prepared installation,
setup and device identities plus a 32-byte base64url token. The ledger stores its
hash; the receipt carries identity and expiry only. The exact completed request
can be confirmed after a lost reply and restart, without renewal or another device.
The setup offer's deadline limits initial completion; a completed retry instead
requires that its original pairing still be active.

Expired, revoked, replaced, conflicting and malformed requests fail. Stop intent
and temporary/bounded sessions prevent setup. Beginning setup never resumes the
host or replaces an active owner. The existing single-owner-device restriction
remains. Native clients must save their pending identity and token before submitting
completion; that storage and transport work is still pending.

A legacy enrollment successfully completing now removes an outstanding owner
setup record. Otherwise that old offer could have been used after the legacy
pairing was disconnected and the owner explicitly resumed the host. Conversely,
owner setup completion consumes outstanding legacy enrollment codes. Neither path
adds an unauthenticated HTTP route. This is a trusted composition API, not a new
network authentication mechanism or computer-tool sandbox.

## Verification

- Red: lost-reply test failed because `beginOwnerSetup` did not exist.
  `/tmp/fort059-owner-setup-red.log`.
- First green: lost-reply/restart/authentication test passed.
  `/tmp/fort059-owner-setup-green.log`.
- Boundary red: legacy enrollment left a reusable pending setup; fixed by
  invalidating that record in the same enrollment transaction. A separate test
  initially attempted HTTP after the existing stop endpoint had closed its listener;
  the test now reopens the core before checking persisted stop behavior.
  Both original failures remain in `/tmp/fort059-owner-setup-boundaries.log`.
- 25 setup/pairing/recovery tests passed, including 11 new setup tests. They cover
  pending and completed restart, exact expiry, invalid token encodings, conflicting
  identities/tokens, three revoked generations, stop/closed states, legacy behavior,
  bounded-session exclusion, authenticated API access, hash-only storage and corrupt
  stored metadata. `/tmp/fort059-owner-setup-tests.log`.
- Registered portable suite: **873 passed**, zero failed/skipped/canceled.
  `/tmp/fort059-owner-setup-portable.log`.
- Strict TypeScript check and full/implementation spec gates passed.
  `/tmp/fort059-owner-setup-typecheck.log`.
- Built `successor/.build/owner-setup-059-20261018`: 26 files, 178,566 bytes.
  All manifest file hashes checked. Manifest SHA256:
  `3ea69cd4c1c4d0276b7b02ef01ecbf27fe2468ea14f8b88750f878c571766be1`.
  A separate compiled-artifact check completed setup, reopened the ledger and
  confirmed the same pairing with an independent runtime descriptor. No provider
  implementation was loaded. Build inventory: `/tmp/fort059-owner-setup-manifest.json`.

No package dependency, timer, HTTP route or background activation was added. Work
uses the existing exclusive ledger and one metadata record. No performance budget
was newly measured; earlier core measurements describe their recorded revisions.
Swift, signed XPC, installed UI and real-provider tests were not rerun for this
core-only slice. Their earlier results are separate evidence, not proof of native
owner setup. Pending next work: private transport, native durable preparation,
ordinary app integration and the real owned-host lifecycle.
