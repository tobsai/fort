# Signed XPC owner setup

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), existing Node implementation
authority. Verified October 18, 2026. [Raw redacted proof and source hashes](owner-setup-xpc.json).
No provider, Keychain, ordinary setup or startup registration was activated.

The pinned XPC protocol now has a separate `setupOwner` selector. A listener must
explicitly supply `OwnerSetupControl`; the default stays unavailable. The same
effective-user and client/service signing requirements apply to begin and complete.
Setup frames and replies are limited to 1 KiB, retain bounded deadlines and use
the same one-operation connection and cancellation machinery. Native pipe and XPC
transports share a validated command codec. Completion verifies the submitted
installation/device at both boundaries. Callback failures return a generic
unconfirmed result, without internal error contents or a token.

Only the signed synthetic core fixture opts in. It prepares one synthetic token,
completes setup, authenticates, confirms the same request after cooperative core
restart and abrupt helper death, and retains the original message receipt/history.
Foreign installation/device/token requests fail. Both operations reject wrong
client and service identities and remain disabled in the ordinary test channel.
The HTTP route remains unauthorized before pairing and absent after authentication.
No entitlement or signing identity changed.

## Verification

- Red: the XPC channel unit test initially failed compilation because the selector
  and explicit composition argument were absent. `/tmp/fort059-owner-setup-xpc-red.log`.
- The first incremental build then reported a concurrency error in the unchanged
  `WorkspaceModelTests.swift` URLProtocol helper. A fresh scratch build compiled
  without that error; the execution connection changed before its tests reported
  completion. No matching build/test process remained. The original logs remain
  `/tmp/fort059-owner-setup-xpc-green.log` and `/tmp/fort059-owner-setup-xpc-fresh.log`.
  The helper was not edited; the incremental diagnostic's cause is not established.
- A complete fresh-directory run and final run both passed **93 Swift tests**.
  Four new test functions cover explicit/default-disabled composition, exact receipt,
  malformed/oversized input, invalid deadlines before transmission, sanitized callback
  errors, foreign identity and cancellation/one-operation enforcement.
  `/tmp/fort059-owner-setup-xpc-swift.log` and
  `/tmp/fort059-owner-setup-xpc-swift-final.log`.
- The final Swift run used the exact compiled core from the retained signed app:
  `successor/.build/owner-setup-xpc-059-20261018/FortSandboxProbe.app/Contents/Resources/Core`.
- Retained signed XPC setup/crash proof passed. Helper crash cleanup **14.66 ms**;
  relaunch **10,154.32 ms**, including the existing launchd throttle; total parent
  work **11,512.02 ms**. These are fixture diagnostics, not installed setup latency.
  `/tmp/fort059-owner-setup-xpc-signed.log`.
- The other **3 signed sandbox scenarios passed serially**: compiled core,
  noncooperative child cleanup and private SQLite with outside/network denial.
  `/tmp/fort059-owner-setup-xpc-signed-regressions.log`.
- Independent checks passed: deep/strict code signature, all **7** exact entitlement
  targets, both **26-file** core copies, **11** recorded PIDs and their recorded
  core/parent groups absent, all four generated containers absent. No forced cleanup.
  Both core manifests have SHA256
  `6c5a0abddaa5ae508eb827e871a0752e16fd48d9f7b0e8e4407971e699277cf8`.
- Strict TypeScript and full/implementation spec gates passed.
  `/tmp/fort059-owner-setup-xpc-typecheck.log`. The last portable suite remains the
  **878-test** pass at the private-pipe slice; it was not repeated for native/XPC
  and signed fixture changes. Guardian behavior last passed its 14 checks at the
  preceding native slice; this slice moved its shared command codec without changing
  process ownership logic.
- Fresh unsigned engineering builds passed:
  `/tmp/fort059-owner-setup-xpc-mac-build/Build/Products/Debug/FortMac.app` and
  `/tmp/fort059-owner-setup-xpc-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.
  Logs have the same directory prefixes plus `.log`. Neither app was installed/launched.

CoreSimulator remains 1171.7.0 versus Xcode's 1174.9.2, with Simulator device support
disabled. No shared service was restarted. Native pending identity/credential storage,
ordinary app composition and real owned-host lifecycle remain unimplemented. Installed
UI/accessibility, actual providers, TestFlight and physical acceptance remain unrun.
The signed helper's agent-host boundary remains distinct from computer-tool containment.
