# Owner setup over the private parent pipe

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), existing Node implementation
authority. Verified October 18, 2026. Extends the [core setup seam](owner-setup.md)
without activating ordinary owner setup, providers, Keychain or startup services.

`servePrivateChannel` accepts an explicit `ownerSetup` composition with begin and
complete callbacks. Without it, setup frames fail closed. This control uses the
existing owned parent pipe, not an HTTP route or bearer-token exception. IDs,
operation fields, canonical 32-byte token encoding and 0–30 second deadlines are
validated before calling composition. Requests share existing monotonic IDs,
eight-operation concurrency, input/frame/output limits and cancellation behavior.

Begin returns a validated offer or existing pairing. Completion must return the
submitted installation/device; only receipt fields are serialized. Callback errors
and malformed replies expose no callback contents. Timeout/cancellation cannot
undo an already committed pairing, so the reply stays uncertain. A retry with the
same prepared identity/token confirms it. The integration fixture deliberately
delays after commit and supplies an extra secret-like field to verify uncertainty
handling and response field selection.

## Verification

- Red: the new setup frame caused the previous channel to exit.
  `/tmp/fort059-owner-setup-pipe-red.log`.
- Green: real core setup, authenticated API access and restart confirmation over
  the parent pipe passed. `/tmp/fort059-owner-setup-pipe-green.log`.
- All **14 private-channel tests passed**, including five new setup cases, source
  and compiled setup journeys, disabled/malformed operations, timeout/cancel after
  commit, wrong callback identity, invalid offer, callback exception, eight matching
  concurrent retries and rejection of a ninth. Existing recovery, forwarding,
  malformed-frame and backpressure tests still pass.
  `/tmp/fort059-owner-setup-pipe-tests.log`.
- Registered portable suite: **878 passed**, zero failed/skipped/canceled.
  `/tmp/fort059-owner-setup-pipe-portable.log`.
- Strict TypeScript and full/implementation spec gates passed.
  `/tmp/fort059-owner-setup-pipe-typecheck.log`.
- Retained compiled artifact: `successor/.build/owner-setup-pipe-059-20261018`,
  26 files, 181,964 bytes. Every manifest file hash verified. Manifest SHA256:
  `6c5a0abddaa5ae508eb827e871a0752e16fd48d9f7b0e8e4407971e699277cf8`.
  Inventory: `/tmp/fort059-owner-setup-pipe-manifest.json`.

Temporary processes, socket directories and ledgers were removed by the tests.
No new dependency, polling loop or background process was added. No new performance
budget measurement was made. Swift, signed XPC, ordinary app setup, persistent
credential preparation, installed UI and providers remain separate, unrun gates
for this operation. The pipe does not supply peer code-signing checks or a
computer-tool sandbox; those remain separate host/executor responsibilities.
