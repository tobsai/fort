# Signed owner setup cancellation

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), under the existing Node
implementation authority. Verified October 18, 2026. The
[redacted proof and source hashes](owner-setup-cancellation-xpc.json) identify this
exact synthetic artifact; no Keychain/provider or startup registration was used.

The signed wrapper now explicitly provides `OwnerSetupCancellationControl` through
its existing pinned host channel. The test calls the Swift transport, XPC wrapper,
owned core pipe and ledger. A pending offer is canceled and cannot later complete;
an explicit new offer creates the synthetic pairing. Exact completed cancellation
then revokes that token. After another core restart, retry returns the same revoked
receipt, begin remains blocked, the retained request receipt is still present and
no execution has restarted. Existing message/replay and abrupt helper recovery
checks remain in the same journey.

All three foreign cancellation identities are rejected without changing the active
pairing. Anonymous HTTP remains unauthorized; authenticated HTTP has no cancellation
route. The generic signed host still has cancellation disabled, and wrong client
and service pins refuse the operation. No entitlement, signing identity, provider,
storage dependency or persistent service was added. This verifies agent-host
transport and fixture confinement separately from computer-tool sandbox authority.

## Evidence

- Red: the updated signed regression failed at missing `xpcSetupCancelPending`.
  The earlier probe completed and cleaned up; it had no cancellation checks.
  `/tmp/fort059-owner-setup-cancellation-xpc-red.log`.
- Green: the updated signed XPC test passed all old and nine new cancellation
  assertions. `/tmp/fort059-owner-setup-cancellation-xpc-green.log`.
- A separate retained signed run also passed, with no forced cleanup:
  `successor/.build/owner-setup-cancellation-xpc-059-20261018/FortSandboxProbe.app`.
  Identifier: `io.fort.foundation.probe.a3724bbd-b8c1-4d45-be4d-c50fbced4ae2`.
  Raw output: `/tmp/fort059-owner-setup-cancellation-xpc-signed.log`.
- Independent inspection passed deep/strict signature verification, all **7** exact
  entitlement sets, both **26-file** compiled-core copies, absence of **13** recorded
  PIDs and **5** process groups, no matching owned executables and absence of all
  **4** exact temporary containers. Both core manifests have SHA256
  `5c7dbb937a80b48ce44e5e922909e08cc9a5f9b2ff120be1986dd031d46c00d5`
  and 186,928 bytes of listed files. The independent record includes 17 source hashes.
- The retained run observed crash cleanup in **14.46 ms** against 3,000 ms and core
  relaunch in **10,153.89 ms** against its separate 12,000 ms limit. Relaunch includes
  the existing launchd throttle; it is not an ordinary startup benchmark. Parent
  work was 11,763.49 ms within the existing 15,000 ms phase bound. Four core
  generations and both helper generations are included in absence checks.
- The other **three signed scenarios passed** serially: compiled core replay,
  hung-child cleanup and sandboxed SQLite/filesystem/network controls.
  `/tmp/fort059-owner-setup-cancellation-xpc-signed-regressions.log`.
- Full native package: **111 test functions passed** after the shared fixture change.
  `/tmp/fort059-owner-setup-cancellation-xpc-swift.log`.
- Strict TypeScript and full/implementation spec gates passed.
  `/tmp/fort059-owner-setup-cancellation-xpc-typecheck.log`.

Portable tests were not rerun for signed-fixture changes; the prior pipe slice
remains 892 passed. Ordinary native app source has not changed since the fresh
unsigned builds recorded in [native cancellation](owner-setup-cancellation-native.md).
Those builds, signed fixture behavior and installed-product acceptance are separate.
Client pending-state recovery/discard and ordinary owned-host composition remain
next work. Actual Keychain, provider behavior, installed accessibility/screenshots,
TestFlight and physical acceptance remain unrun; CoreSimulator remains incompatible.
