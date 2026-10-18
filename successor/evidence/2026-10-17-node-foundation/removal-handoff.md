# Native removal handoff

Under [Spec 059](../../../specs/059-modular-node-v1.md), native removal now
prepares the app for supported macOS/TestFlight removal. It does not delete its
own bundle or report that Fort has already been removed. Source base: `2d4d296`.

The lifecycle requires three independent confirmations before removing the local
credential: host stopped, pairing revoked and startup unregistered. The returned
preparation identifies the exact app URL and whether history was kept or explicitly
deleted. Failed credential/history cleanup produces no ready result; retry remains
possible from retained disconnect information. History is kept by default. Every
active caller was migrated from `uninstall` to `prepareForRemoval`; the old
`removeApp` protocol method was deleted. The native settings text explains the
remaining system-removal step.

Verification:

- The first public lifecycle regression failed because the old implementation
  invoked app removal. After migration, a second regression failed because an
  unconfirmed startup registration still allowed credential removal. Both passed
  after the correction.
- All 63 Swift package tests passed, including missing stop/revoke/unregister
  confirmations, failed credential/history cleanup, retry, and native model
  presentation of a ready handoff only after confirmed cleanup.
- Fresh unsigned Debug Mac and generic iPhone Simulator builds passed. Products:
  `/tmp/fort059-removal-mac/Build/Products/Debug/FortMac.app` and
  `/tmp/fort059-removal-ios/Build/Products/Debug-iphonesimulator/FortiOS.app`.
  Local package inputs only; no build scripts or remote package dependencies were
  found in the generated project. Xcode reported the existing CoreSimulator
  service/version mismatch and skipped unused AppIntents metadata extraction.
  These builds do not establish Simulator boot or installed UI acceptance.
- Full structural and accepted/staged spec gates and whitespace checks passed.
  Node, provider, signed-bundle and installed UI checks were not repeated for
  this native contract slice.

The real owned-host/registration adapter remains absent, so ordinary construction
still disables removal preparation. These tests use lifecycle and credential
fixtures. They do not unregister any service, remove any installed application,
delete real history or access Keychain. Actual process absence, startup cleanup,
installed accessibility/screenshots and TestFlight removal remain open.

Apple's local Xcode 27.1 SDK `ServiceManagement.framework/Headers/SMAppService.h`
states that synchronous unregistration does not wait for process exit; the
asynchronous completion waits for the managed process to be killed. Neither
assertion substitutes for Fort's owned descendant-process absence evidence.

Logs: `/tmp/fort059-removal-red.log`,
`/tmp/fort059-removal-registration-red.log`, `/tmp/fort059-removal-green.log`,
`/tmp/fort059-removal-mac-build.log`, `/tmp/fort059-removal-ios-build.log`.
