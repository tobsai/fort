# Owner setup cancellation over the private pipe

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), under the existing
[Node decision](../../../docs/architecture/decisions/059-node-foundation.md).
Verified October 18, 2026 with synthetic owned processes and temporary ledgers.

The `ownerSetup` frame now accepts `operation: cancel`, the exact installation,
setup and device IDs, and a deadline. It rejects token/extra fields and malformed
identities before invoking core. The cancellation callback is optional and remains
unavailable if absent. Existing compositions that only begin/complete setup retain
that behavior.

The channel checks all three receipt identities and the distinct `notPaired` or
`revoked` outcome. Revocation requires `runtimeStopped: true`; only known fields
cross the pipe. Callback errors and additional fields cannot expose their contents.
Cancellation uses the existing monotonic frame IDs, eight-operation admission
limit, frame/body bounds, request deadline and five-second output drain bound.

Timeout/cancel replies mean uncertainty, including when core already committed
cancellation. Exact retries confirm the durable outcome after reopening. They do
not complete setup, renew access or resume the host. The core operation and its
limits are documented in [core cancellation](owner-setup-cancellation.md).

## Verification

- Red: the source journey failed because the pipe rejected the new operation and
  exited. `/tmp/fort059-owner-setup-cancellation-pipe-red.log`.
- Green: pending cancellation, delayed completion refusal, a separate completed
  setup's revocation, and restart retry passed through the private pipe.
  `/tmp/fort059-owner-setup-cancellation-pipe-green.log`.
- **20 pipe tests passed**, including six new cancellation checks. Tests cover
  source and compiled journeys, malformed identity/token/deadline fields with zero
  callback invocations, absent callback, all three foreign receipt identities,
  unconfirmed shutdown, invalid outcomes, secret-free errors, timeout/canceled
  replies for both outcomes, exact retry after restart, and eight admitted
  cancellation calls with the ninth rejected. Existing backpressure, EOF, auth,
  recovery and setup checks remain passing. Fixture cleanup checks owned process
  group absence; no provider or persistent registration is involved.
  `/tmp/fort059-owner-setup-cancellation-pipe-tests.log`.
- Registered portable suite: **892 passed**, zero failed/skipped/canceled.
  `/tmp/fort059-owner-setup-cancellation-pipe-portable.log`.
- Strict TypeScript and full/implementation spec gates passed.
  `/tmp/fort059-owner-setup-cancellation-pipe-typecheck.log`.
- Retained compiled core:
  `successor/.build/owner-setup-cancellation-pipe-059-20261018`,
  26 files, 186,928 bytes. Every manifest file size/hash matched. Manifest SHA256:
  `5c7dbb937a80b48ce44e5e922909e08cc9a5f9b2ff120be1986dd031d46c00d5`.
  Inventory: `/tmp/fort059-owner-setup-cancellation-pipe-manifest.json`.
  The behavioral suite built and exercised a separate temporary compiled artifact
  with JIT and TypeScript stripping disabled.

No dependencies, HTTP route, idle work, new authority or provider were added.
This is an agent-host channel, not computer-tool confinement. Swift/XPC cancellation,
native pending recovery/discard and ordinary app composition remain unimplemented.
Swift, signed packaging, native builds, installed UI, real Keychain and real providers
were not rerun for this pipe-only slice. Earlier results remain separate evidence;
no installed-product or TestFlight acceptance is claimed.
