# Native owner setup through the owned core process

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), existing Node implementation
authority. Verified October 18, 2026. Uses synthetic prepared credentials; no
Keychain, provider, ordinary setup or startup registration was activated.

FortKit now owns validated setup offer, request, receipt and state values, plus
`OwnerSetupControl`. These types use the same installation/setup/device fields as
the core and remain available to non-Mac transports. `MacCoreProcessTransport`
implements begin/complete through the [private pipe](owner-setup-pipe.md), retaining
its request credit, cancellation, deadline and process lifecycle behavior.

Decoding validates bounded identities, canonical 32-byte base64url tokens and
canonical UTC expiry strings. A receipt explicitly includes either null expiry
or a valid expiry, and must match the completion's exact installation/device.
Malformed or foreign output never becomes a native success receipt. The API
does not generate a token or write storage; durable preparation remains a separate
client responsibility before submission.

## Verification

- Red: the new native test did not compile because the setup interface was absent.
  `/tmp/fort059-owner-setup-native-red.log`.
- Green: Swift receives an offer, completes setup with a prepared synthetic token,
  authenticates, closes/reopens the compiled core and confirms the same pairing.
  `/tmp/fort059-owner-setup-native-green.log`.
- **89 Swift tests passed**, including five new test functions. Their parameterized
  cases cover deadline/cancellation after a known commit, exact retry, malformed
  offer/receipt/status, foreign identities, token encoding, expiry validation,
  round-trip Codable values and invalid deadlines before transmission. Tests
  independently verify each owned core PID/group and socket is absent after close.
  `/tmp/fort059-owner-setup-native-tests.log`.
- Swift exercised the exact retained compiled core at
  `successor/.build/owner-setup-pipe-059-20261018` (26 files; manifest SHA256
  `6c5a0abddaa5ae508eb827e871a0752e16fd48d9f7b0e8e4407971e699277cf8`).
- Both standalone Swift compiler lists include `OwnerSetup.swift`.
  All **14 guardian checks passed**: abrupt Swift and Node parent death, child
  groups, normal exit, cancellation, permission denials and existing warm/idle
  budgets. `/tmp/fort059-owner-setup-native-guardian.log`. These remain short
  guardian-only measurements, not whole installed-host performance.
- All **4 signed sandbox regression scenarios passed serially**, using the existing
  selected development identity. They retain exact XPC peer checks, sandbox denials,
  helper crash/restart and owned-process/container absence. No entitlement changed.
  `/tmp/fort059-owner-setup-native-signed.log`. Test bundles were removed on success;
  the earlier retained crash-recovery bundle remains evidence of its earlier revision.
  These scenarios do not yet exercise setup through signed XPC.
- Strict TypeScript, full spec and implementation gates passed.
  `/tmp/fort059-owner-setup-native-typecheck.log`. The last registered portable
  run remains **878 passed** at the private-pipe slice; it was not rerun for Swift
  types and standalone compiler-list changes.
- Fresh unsigned Mac and iPhone Simulator engineering builds passed:
  `/tmp/fort059-owner-setup-mac-build/Build/Products/Debug/FortMac.app` and
  `/tmp/fort059-owner-setup-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.
  Logs: `/tmp/fort059-owner-setup-native-mac-build.log` and
  `/tmp/fort059-owner-setup-native-ios-build.log`.

Both builds still report CoreSimulator 1171.7.0 versus Xcode's required 1174.9.2;
Simulator device support is disabled. No shared service was restarted. Build
success does not prove Simulator execution, UI/accessibility, TestFlight or physical
acceptance. No ordinary app was installed or launched.

Remaining: signed XPC setup forwarding, durable pending identity/credential
storage before completion, ordinary app composition, real host lifecycle and
installed/provider/distribution acceptance. Setup is not auto-enabled in the
signed fixture or ordinary app by this slice. The agent host and computer-tool
sandbox remain separate security boundaries.
