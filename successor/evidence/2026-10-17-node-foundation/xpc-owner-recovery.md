# Pinned Mac owner recovery

Governing design: [Spec 059](../../../specs/059-modular-node-v1.md). Source base
`d5dc065`; exact inputs and artifact results are in [xpc-owner-recovery.json](xpc-owner-recovery.json).
This is synthetic native/packaging evidence, not installed removal or provider
acceptance.

The XPC protocol has a distinct `recoverPairing` method. `MacHostListener` still
requires the exact client signing identity/team and effective user; the client
pins the exact service identity/team. Recovery remains unavailable unless the
host explicitly supplies a `MacOwnerControl`. It never passes through HTTP and
does not accept a bearer-token exception.

Both peers validate bounded installation/device IDs and a deadline of at most
30 seconds. Recovery requests/responses are limited to 1 KiB. A connection admits
one operation; invalidation cancels its task. Both peers reject a receipt unless
it names the exact requested pairing and confirms runtime disposal, pairing
revocation and core-process exit. The request type is shared with the native
process transport so those limits stay consistent.

The signed fixture supplies the real Node recovery callback, confirms clean
process/group exit through `MacCoreProcessTransport.close`, and returns the
receipt through the production listener/client. Synthetic bootstrap and restart
remain fixture-only. The old fixture HTTP stop route was removed; its signed
journey now uses the real owner-control method. The fixture exits its helper
after replying; the external runner verifies helper and group absence separately.

Passed:

- The new signed regression first failed because `xpcOwnerRecovery` was absent.
  After implementation it passed with zero skips, including both signing mismatch
  directions, explicitly disabled control, foreign installation/device rejection,
  no HTTP recovery route, durable messaging/restart, and every prior sandbox and
  process-cleanup control.
- Three new native tests cover disabled control, malformed/oversized input,
  incomplete/foreign receipts, one-operation admission and cancellation. All 84
  native tests passed against the compiled core, including expired-token recovery
  and history preservation over the actual Swift/Node pipe.
- Strict TypeScript checking, full/implementation spec gates and fresh unsigned
  Mac/iPhone Simulator builds passed. Builds:
  `/tmp/fort059-xpc-owner-mac/Build/Products/Debug/FortMac.app` and
  `/tmp/fort059-xpc-owner-ios/Build/Products/Debug-iphonesimulator/FortiOS.app`.
- Retained signed artifact:
  `successor/.build/xpc-owner-059-20261018/FortSandboxProbe.app`.
  Independent checks confirmed deep/strict signature, all six unchanged
  entitlement targets, all 22 core file hashes, owned PID/group absence and
  removal of generated containers. No forced XPC cleanup was needed. The core
  manifest remains `4bdc218107386e8592dbaf0e3b2fb59e66ff9d01d73f24a485a83098e9cc249b`.

Logs: `/tmp/fort059-xpc-owner-{red,unit-first,first,typecheck,swift,mac-build,ios-build,artifact}.log`.
The other three signed scenarios and 848 portable tests passed in preceding
slices; they were not repeated for this Mac-only control change.

CoreSimulator still reports running version 1171.7.0 versus build 1174.9.2 and
disables device support. Simulator execution remains unverified; the shared
service was not restarted. Ordinary app setup and the real owned installation
adapter remain absent. A recovery receipt alone cannot establish helper exit or
startup unregistration. Installed accessibility/screenshots, real providers,
TestFlight and physical acceptance remain unrun. No real credential, provider
session, account, startup registration or installed app was changed.
