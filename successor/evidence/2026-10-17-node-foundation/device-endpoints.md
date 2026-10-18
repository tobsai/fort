# Native saved endpoint migration

Saved native connections now select a network endpoint or an exact Mac XPC
service identifier/signing team. Restore and disconnect recovery use that carrier
without a network placeholder or fallback. The connection record is version 2;
its installation/device identifiers and existing Keychain account construction
are unchanged. Governing design: [Spec 059](../../../specs/059-modular-node-v1.md).
Source base: `66cee4a` plus this slice.

The decoder still accepts the previous unversioned URL record, preserving its
URL, fixture-loopback choice and identities. Reading it neither rewrites metadata
nor reads a credential. The next explicit save emits the new format. Unknown
versions/kinds, mixed carrier fields, insecure network endpoints and malformed
Mac peer identities fail before credential access. Encoding happens before a
credential save. Identity validation rejects empty, control-bearing and
separator-ambiguous identifiers. Transport selection precedes token lookup.

Disconnected local history and removal intents retain the endpoint and exact
pairing identity without starting a host or reading a token. A Mac endpoint is
explicitly unavailable on iPhone; no alternative carrier is selected. Failed
restore keeps the saved pairing rather than instructing automatic reenrollment.
The existing network enrollment flow remains separate from future owned setup.

Verification:

- Red: the migration test failed because `DeviceConnection` had no endpoint.
  A throwing-optional test macro was corrected before the green run.
- All 79 Swift package tests passed. New checks cover byte-preserving legacy
  reads, current round trips, local disconnected history, retained removal
  identity, malformed/mixed/unknown data and zero credential operations after
  invalid input. Existing network, recovery, native process and server tests pass.
- Fresh unsigned Mac and generic iPhone Simulator builds passed, respectively at
  `/tmp/fort059-device-endpoint-mac/Build/Products/Debug/FortMac.app` and
  `/tmp/fort059-device-endpoint-ios/Build/Products/Debug-iphonesimulator/FortiOS.app`.
  The latter log reports running CoreSimulator 1171.7.0 older than build 1174.9.2,
  with Simulator device support disabled. This is compile evidence only; no
  Simulator execution or platform-specific local-host error presentation passed.
- The signed XPC/core scenario passed again after peer-identity validation was
  made available to both platform decoders. It retains the direct host denials,
  signature mismatches, messaging/restart and process/container cleanup controls.
- Structural, accepted/staged spec and whitespace checks passed before commit.
  Portable Node tests were not repeated for this native-only change.

All migration data and credential providers were synthetic and temporary. No
existing connection, Keychain item, provider account, startup registration or
installed app was changed. The ordinary app can now represent the local carrier;
real host setup, lifecycle, reachable iPhone networking and installed/TestFlight
acceptance remain open. The prior [signed artifact](xpc-core.md) remains pinned to
its own source revision rather than being relabeled as this build.

Logs: `/tmp/fort059-device-endpoint-red.log`, `-green.log`, `-mac-build.log`,
`-ios-build.log` and `-xpc.log` under the same prefix.
