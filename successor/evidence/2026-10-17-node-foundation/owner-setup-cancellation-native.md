# Native owner setup cancellation

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), under the existing
[Node decision](../../../docs/architecture/decisions/059-node-foundation.md).
Verified October 18, 2026 with synthetic storage/credentials and owned fixtures.
No actual Keychain, provider, persistent registration or ordinary owner setup was
activated.

`OwnerSetupIdentity` validates all three identities. `OwnerSetupCancellation` keeps
`notPaired` and `revoked` distinct; revoked requires confirmed runtime shutdown on
decode, while not-paired cannot carry a shutdown assertion. The separate optional
`OwnerSetupCancellationControl` capability keeps existing begin/complete controls
from gaining cancellation by inference.

The owned core and pinned XPC transports implement cancellation through the existing
setup command. Both correlate the exact identity before returning a receipt. The
host channel refuses cancellation without its explicit capability, rejects unknown
or credential fields before invoking control, and serializes typed results. Errors
are sanitized. Existing request/response limits, deadlines, peer pins and one-operation
host-channel admission remain unchanged. No receipt claims helper absence, startup
unregistration or app removal.

## Verification

- Red: the Swift-to-core regression failed to compile because cancellation values
  and methods were absent. `/tmp/fort059-owner-setup-cancellation-native-red.log`.
- Green: both pending and completed setup cancellation confirmed the same result
  after compiled core restart.
  `/tmp/fort059-owner-setup-cancellation-native-green.log`.
- Targeted native suite: **27 test functions passed**, including parameterized
  pending/completed, timeout/task-cancel and eight malformed/foreign proof cases.
  Additional host-channel checks cover missing capability, all three receipt identity
  mismatches, invalid fields/deadlines, secret-free errors, admission and invalidation.
  `/tmp/fort059-owner-setup-cancellation-native-targeted.log`.
- Full Swift package: **111 test functions passed**.
  `/tmp/fort059-owner-setup-cancellation-native-swift.log`.
  Core-backed native tests used the retained 26-file compiled pipe artifact with
  manifest SHA256 `5c7dbb937a80b48ce44e5e922909e08cc9a5f9b2ff120be1986dd031d46c00d5`.
  The artifact path and build conditions are recorded in [pipe evidence](owner-setup-cancellation-pipe.md).
  Each native core cleanup confirms clean exit, no forced stop, PID/process-group
  absence and removal of the exact temporary socket.
- **14 guardian checks passed**, including native parent exit and executor controls.
  `/tmp/fort059-owner-setup-cancellation-native-guardian.log`.
- **Four existing signed sandbox scenarios passed serially** using the already
  authorized development identity. They preserve exact peer rejection, sandbox
  denials, setup/recovery, abrupt helper recovery and owned-process/container cleanup.
  They do not exercise end-to-end signed cancellation yet.
  `/tmp/fort059-owner-setup-cancellation-native-signed.log`.
- Strict TypeScript and full/implementation spec gates passed.
  `/tmp/fort059-owner-setup-cancellation-native-typecheck.log`.
- Fresh unsigned Mac and iPhone Simulator engineering builds passed:
  `/tmp/fort059-owner-setup-cancellation-mac-build/Build/Products/Debug/FortMac.app`
  and `/tmp/fort059-owner-setup-cancellation-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.
  Logs: `/tmp/fort059-owner-setup-cancellation-native-{mac,ios}-build.log`.
  CoreSimulator remains incompatible: installed 1171.7.0 versus required 1174.9.2;
  device support was disabled. No Simulator service was restarted or changed.

The last portable suite remains 892 passed at the pipe slice; it was not rerun for
these Swift bridges and native fixtures. No performance measurement is inferred
from build/test durations. The signed fixture wrapper still needs to opt into and
exercise the new cancellation capability. Native pending-record discard/recovery,
ordinary app composition and owned startup lifecycle remain next work. Installed
screenshots/accessibility, actual Keychain/providers, TestFlight and physical
acceptance remain unrun.
