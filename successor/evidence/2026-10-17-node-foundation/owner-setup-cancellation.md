# Owner setup cancellation

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), under the existing
[Node decision](../../../docs/architecture/decisions/059-node-foundation.md).
Verified October 18, 2026 in the isolated implementation worktree. This is a
trusted local core operation; no provider, Keychain, startup registration or
ordinary owner activation was used.

## Behavior

`cancelOwnerSetup` takes the exact installation, setup and device identities.
It does not need the token that may have been lost during interrupted preparation.
A pending offer is durably canceled before returning `notPaired`. A late completion
cannot use that offer, including after restart. Only an explicit begin can replace
the canceled offer. An obsolete offer can be confirmed not paired only when its
requested device is absent; it cannot revoke a newer or legacy pairing.

For completed setup, the ledger atomically records cancellation and disconnect
intent before disposal. The existing owner stop boundary retains pairing until
runtime shutdown is confirmed. Failure retains both the intent and pairing for
retry; restart does not resume work. Even if separate pairing recovery removes the
device, explicit resume remains blocked until setup cancellation finishes. Success
returns `revoked` and `runtimeStopped: true`. A repeated cancellation after reopening
the core confirms disposal again rather than returning historical shutdown proof.

Neither outcome proves helper absence, startup unregistration or app removal.
History is retained. Temporary/bounded sessions refuse the operation. Wrong
identities and malformed stored state fail without exposing stored contents or
runtime error details. No HTTP cancellation route was added. Agent-host authority
remains separate from any computer-tool sandbox.

## Verification

- Pending cancellation red: missing `cancelOwnerSetup` method.
  `/tmp/fort059-owner-setup-cancellation-red.log`.
- Pending green: all 12 then-current setup tests passed.
  `/tmp/fort059-owner-setup-cancellation-pending.log`.
- Completed cancellation red: the first implementation rejected completed setup
  instead of entering durable shutdown recovery.
  `/tmp/fort059-owner-setup-cancellation-completed-red.log`.
- Completed green: 27 then-current setup/pairing/recovery checks passed.
  `/tmp/fort059-owner-setup-cancellation-green.log`.
- Final targeted suite: **33 passed**, including 19 setup tests. Coverage includes
  both completion/cancellation orders, lost replies, restart, held and failed
  disposal, concurrent recovery, no early revocation, expiry, paused/closed hosts,
  corrupt records, superseded/conflicting identities, legacy pairing, bounded
  refusal and history after a new pairing.
  `/tmp/fort059-owner-setup-cancellation-tests.log`.
- Registered portable suite: **886 passed**, zero failed/skipped/canceled.
  `/tmp/fort059-owner-setup-cancellation-portable.log`.
- Strict TypeScript and full/implementation spec gates passed.
  `/tmp/fort059-owner-setup-cancellation-typecheck.log`.
- Fresh compiled artifact: `successor/.build/owner-setup-cancellation-059-20261018`,
  26 files, 185,615 bytes. Every manifest file size/hash matched. Manifest SHA256:
  `2d2c08afdbaffc3d86ba19e8478249d131cc2a573c2ff119fdb63a23548a3b1f`.
  A separate compiled-only Node invocation with JIT/type stripping disabled
  verified pending cancellation, delayed completion refusal, completed revocation
  and restart retry with an independent synthetic runtime descriptor.
  `/tmp/fort059-owner-setup-cancellation-manifest.json` and
  `/tmp/fort059-owner-setup-cancellation-compiled.log`.

No dependency, timer, idle work or activation was added. The operation uses the
existing indexed ledger and runtime stop boundary. No new performance measurement
or arbitrary-power-loss claim is made. Existing owner stop/transport limits remain.
Swift, signed XPC, native builds, installed UI and real providers were not rerun
for this core-only slice; their earlier results remain separate evidence. Private
pipe/XPC cancellation, durable native discard/recovery and ordinary app composition
remain next work. No v1 or TestFlight completion is claimed.
