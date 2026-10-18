# Durable native owner setup preparation

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), existing Node implementation
authority. Verified October 18, 2026. Tests inject synthetic credential storage
and token generators. No actual Keychain access, credential generation, provider,
ordinary owner activation or startup registration occurred.

`OwnerSetupCoordinator` binds setup to one exact signed Mac host endpoint. It writes
a private `preparing` record before creating a credential through the existing
device credential abstraction. After that write succeeds, it writes `ready` with
the token's SHA256 digest before calling core completion. A retry reads the same
identity/token; a missing ready credential is never replaced, and an altered token
is rejected before submission. A credential write that succeeded before reporting
an error is recovered by reading and reusing that credential while still preparing.

The coordinator verifies the receipt identity, unchanged preparation and retained
token, then saves `connection.json` durably before removing the pending record.
Failures leave preparation available for retry. Existing foreign connections and
host pairings are preserved. A matching saved connection is returned as existing
configuration, without reenrollment or a claim that its host is running. Historical
navigation, messages, disconnected records and removal intents are untouched.

`OwnerSetupFiles` holds a nonblocking exclusive file lock across the operation.
Its directory and regular-file checks enforce current owner/private modes and
reject symlinks, hardlinks and special files. Reads are bounded to 16 KiB and
anchored to an open directory descriptor. Atomic replacements use mode 0600,
file `F_FULLFSYNC` and directory `fsync`; pending deletion follows the durable
connection write. Unknown/corrupt records are preserved. No polling or idle worker
was added. Hard crashes may leave unreferenced private temporary files; these are
not guessed at or automatically deleted. Actual hardware power-loss behavior is
not proven by the process/reopen fixtures.

## Verification

- Red: the preparation test failed compilation because the coordinator was absent.
  `/tmp/fort059-owner-setup-preparation-red.log`.
- The first implementation had a missing Swift `try` marker; corrected without
  changing test expectations. `/tmp/fort059-owner-setup-preparation-green.log`.
  Initial lost-reply/reopen behavior then passed:
  `/tmp/fort059-owner-setup-preparation-first-pass.log`.
- A new regression demonstrated that an altered ready credential could still be
  submitted. The ready record now binds its digest and rejects this locally.
  `/tmp/fort059-owner-setup-preparation-hash-red.log` and
  `/tmp/fort059-owner-setup-preparation-credentials.log`.
- **104 Swift tests passed**, including 11 new preparation test functions. Cases
  cover partial credential writes, read denial, invalid generation, lost replies,
  missing/altered ready credentials, foreign host/receipt, concurrent coordinators,
  cancellation after commit, connection-write failure, interrupted finalization,
  retained history, existing owner refusal and nine malformed/unsafe-file cases.
  `/tmp/fort059-owner-setup-preparation-tests.log` and
  `/tmp/fort059-owner-setup-preparation-swift.log`.
- An actual compiled-core integration completes pairing, deliberately loses the
  completion reply, closes/reopens the core and recreates client storage/coordinator,
  then confirms the original credential and authenticates. Both core process groups
  and private sockets are independently checked absent after closing. It uses the
  26-file core in the retained [signed XPC app](owner-setup-xpc.md), manifest SHA256
  `6c5a0abddaa5ae508eb827e871a0752e16fd48d9f7b0e8e4407971e699277cf8`.
- Fresh unsigned engineering Mac and iPhone Simulator builds passed:
  `/tmp/fort059-owner-setup-preparation-mac-build/Build/Products/Debug/FortMac.app`
  and `/tmp/fort059-owner-setup-preparation-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.
  Logs use those directory prefixes plus `.log`. No ordinary app was installed/launched.
  CoreSimulator still reports 1171.7.0 versus required 1174.9.2, with device support
  disabled; no service was restarted.
- Full/implementation spec gates passed. Node/TypeScript core, XPC transport and
  signing configuration did not change in this slice. Their last checks remain
  878 portable tests and the four signed scenarios recorded in earlier setup evidence;
  those suites and TypeScript checking were not repeated for these Mac-only sources.

## Local preparation timing

MacBookPro18,2, 10 CPU cores, 64 GiB RAM; macOS 27.2/26B5112a;
Xcode 27.1/27A9275; Swift 6.4 (swiftlang-6.4.0.34.1, clang-2100.3.34.1).
One Swift test process, warm compiled executable, 20 fresh private directories
(created before each timer), real file sync/rename and in-process synthetic
credential/control implementations. OS caches were not cleared. No XPC, real
Keychain prompt or provider cost is included. The isolated filtered run's
nearest-rank p95 was **22.06 ms**, within the **500 ms** target.

Samples in milliseconds: 19.019, 18.177, 17.178, 16.667, 18.352, 17.548,
16.498, 18.388, 16.410, 16.416, 18.352, 16.647, 17.494, 16.619, 17.230,
22.059, 18.064, 23.807, 18.715, 18.995.

Raw isolated measurements: `/tmp/fort059-owner-setup-preparation-budget.log`.
Earlier runs are retained: 21.51 ms p95 alongside the focused preparation tests,
and 40.31 ms during the full concurrent Swift suite. These are different test-load
conditions, not an optimization comparison or installed setup measurement.

The public coordinator is available only on Mac and is not called by ordinary app
startup/restore. Its production credential factory and Keychain behavior remain
unrun. Next: explicit pending-setup recovery/discard presentation, ordinary app
composition and verified owned-host lifecycle. Missing ready credentials remain
blocked for recovery, rather than silently replacing access. Installed Keychain,
UI/accessibility, real-provider, TestFlight and physical acceptance remain open.
