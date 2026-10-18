# Explicit native owner setup flow

Scope: [Spec 059](../../../specs/059-modular-node-v1.md), under the existing
[Node decision](../../../docs/architecture/decisions/059-node-foundation.md).
Verified October 18, 2026 using synthetic credentials, control and transport.
No actual Keychain, provider account or startup registration was used.

`MessengerModel` now offers an explicit Mac-only construction path with a pinned
host service identifier and signing team. Ordinary construction remains unchanged:
`Apps/FortApp.swift` still has no production host bundle or lifecycle adapter and
therefore does not enable these controls. This is an opt-in source flow, not a
working installed owner setup claim.

When configured, restore and sheet inspection read validated pending metadata
without credential access or a host request. Unfinished or unsafe setup blocks
normal restoration, including a connection saved before completion was interrupted.
Explicit connect/retry invokes the existing durable coordinator. Its successful
connection uses the ordinary `FortClient` via the endpoint transport boundary.
An internal injectable transport permits model tests without contacting XPC.

Cancellation captures the displayed installation/setup/device identity. A changed
confirmation cannot target a replacement setup. Cancellation-in-progress disables
completion; confirmed remote cancellation with incomplete local cleanup offers
that cleanup separately. Lost replies and errors retain recoverable state. During
an action, repeated clicks, inspection, restore and legacy fixture enrollment
cannot race it. A configured host cannot restore a connection for a different
endpoint. Successful cancellation keeps saved history and clears stale failure
presentation; it makes no helper-unregistration or app-removal claim.

The Mac connection form and pairing settings project these actions with a progress
state, explicit initial cancellation confirmation, retry/cleanup controls and
accessibility identifiers. iPhone keeps the existing connection form; unconfigured
models, previews and interactive fixtures do not infer a local host. The actual
views have compilation evidence only, not installed screenshot/accessibility proof.

## Verification

- Red: the first model test failed to compile because the explicit host/control/
  transport construction path was absent. `/tmp/fort059-owner-setup-model-red.log`.
- First green: inspection made zero host/credential calls; explicit setup prepared
  one credential and connected through the ordinary client. `...-first-green.log`.
- Boundary red: a configured model restored a saved connection for another host,
  read its token and invoked transport. The saved endpoint now must match the
  configured host before restoration. `...-boundaries.log`; eight model functions
  then passed in `...-boundaries-green.log`.
- Presentation red: successful cancellation left a previous setup error visible.
  The successful no-pending outcome is now presented after restoration.
  `...-presentation.log`; regression passes in the final full suite.
- **140 Swift test functions passed**, including **13 model functions** with
  parameterized host failure and unsafe-record cases. They cover explicit setup,
  lost completion and reopen, exact token reuse, lost cancellation, confirmed
  cleanup retry, interrupted finalization/history preservation, concurrent action/
  task cancellation, legacy-enrollment exclusion, stale confirmation, foreign host,
  future/symlink metadata, unconfigured/preview exclusion, absent cancellation
  capability and ordinary existing-connection restoration.
  `/tmp/fort059-owner-setup-model-swift.log`.

Command from the isolated worktree:

```sh
FORT_TEST_NODE=/opt/homebrew/opt/node@26/bin/node \
FORT_TEST_CORE_ARTIFACT="$PWD/successor/.build/owner-setup-cancellation-pipe-059-20261018" \
swift test --package-path successor/apple/FortKit \
  --scratch-path /tmp/fort059-owner-setup-xpc-native
```

The compiled core used by existing regression tests remains 26 files / 186,928
bytes, manifest SHA256
`5c7dbb937a80b48ce44e5e922909e08cc9a5f9b2ff120be1986dd031d46c00d5`.
No Node, host transport or signed wrapper changed. Prior portable/guardian/signed
results are retained separately, not rerun claims.

## Inspection cost

MacBookPro18,2 / M1 Max / 64 GiB / 10 CPU, macOS 27.2 (26B5112a), Xcode 27.1
(27A9275), Swift 6.4. One isolated run, after the full suite and before the fresh
builds, with 20 separately constructed models reading the same ready pending
record from private temporary storage. No system-cache purge. Construction and
uncertain setup preparation are excluded; timing covers synchronous inspection,
lease acquisition/release, metadata validation and model projection. Credentials,
host control and network calls remain unchanged at zero during those inspections.
p95 is the nineteenth sorted observation; target <=50 ms. This workload proves
warm local metadata inspection, not cold installation, XPC, Keychain or provider
latency. `/tmp/fort059-owner-setup-model-budget.log`.

Measured p95: **0.492667 ms**.

```json
{
  "p95Ms": 0.492667,
  "budgetMs": 50,
  "samplesMs": [
    0.486667,
    0.463667,
    0.452792,
    0.464958,
    0.440583,
    0.44125,
    0.436875,
    0.43725,
    0.440375,
    0.833625,
    0.492667,
    0.447709,
    0.442834,
    0.429791,
    0.425459,
    0.425584,
    0.427917,
    0.479542,
    0.440042,
    0.434125
  ]
}
```

## Builds and remaining work

Fresh unsigned engineering builds passed with `CODE_SIGNING_ALLOWED=NO`:

- Mac: `/tmp/fort059-owner-setup-model-mac-build/Build/Products/Debug/FortMac.app`.
- iPhone Simulator: `/tmp/fort059-owner-setup-model-ios-build/Build/Products/Debug-iphonesimulator/FortiOS.app`.

Commands used the existing `FortSpec058.xcodeproj`, Debug configuration, schemes
`FortMac` / `FortiOS` and generic macOS / iOS Simulator destinations. Logs:
`/tmp/fort059-owner-setup-model-{mac,ios}-build.log`. CoreSimulator remains outdated:
1171.7.0 installed, 1174.9.2 required, device support disabled. No Simulator service
or permissions were changed. Neither app was installed or run.

Full and implementation spec gates passed; exact staged coverage is required before
commit. Production host packaging/configuration, startup lifecycle and real owned
installation adapter remain open. Prior-history-reference reconciliation, reachable
iPhone transport and actual provider adapters remain incomplete. Actual Keychain,
installed accessibility/screenshots, TestFlight and physical acceptance are unrun.
No available ordinary desktop screenshot/accessibility tool was found; the Codex
screen-context tool is restricted to an active voice chat and was not called.
