# Fort CarPlay

Fort's **CarPlay** surface — a glanceable, driving-safe view of the Fort control
plane. It is built from CarPlay's template API (`CPTabBarTemplate`,
`CPListTemplate`, `CPAlertTemplate`), **not** SwiftUI, because CarPlay only
renders Apple's system templates.

Two tabs, few taps:

- **Gates** — every gate awaiting a human decision is one row. Tap a row to get
  an **Approve / Reject** alert; deciding calls `FortClient.decideGate`.
- **Status** — the glanceable `/api/summary` counts (running, queued, blocked,
  succeeded, failed, total) and the execution-plane mode.

Both tabs refresh on connect and on a 5-second timer.

## Files

| File | What it is |
| --- | --- |
| `FortCarPlaySceneDelegate.swift` | The `CPTemplateApplicationSceneDelegate` that builds and refreshes the tab bar and handles gate decisions. |
| `Info-CarPlay-notes.md` | Entitlement + `Info.plist` scene-manifest wiring (project config, not code). |
| `README.md` | This file. |

## Dependency: FortKit

This surface imports **[`FortKit`](../FortKit)** — the shared Swift package that
every Fort Apple surface uses — for the wire models (`Summary`, `GateItem`, …)
and the control-plane client (`FortClient`). It does **not** redefine any of
them. When the Go contract at [`ui/contract.go`](../../contract.go) changes,
update FortKit, not this surface.

## Control-only mode

If Fort runs **control-only** (no deterministic execution plane), gate decisions
can't be applied: `POST /api/gate` returns HTTP 409, and `FortClient.decideGate`
returns `false` (it does **not** throw). The scene surfaces this as a non-fatal
**"No execution plane"** notice rather than an error. The same signal shows on
the Status tab: *Mode → Control-only (no engine)*, which mirrors
`Summary.execution == false`.

## Adding it as an Xcode target

CarPlay ships **inside an iOS app** — there is no standalone CarPlay app. So this
scaffold is added to the Fort iOS app target (or a new iOS app target if one
doesn't exist yet):

1. **Add the source.** In Xcode, add `FortCarPlaySceneDelegate.swift` to your iOS
   app target (drag it in, or *File ▸ Add Files…* and check the app target under
   *Target Membership*).
2. **Link FortKit.** *File ▸ Add Package Dependencies… ▸ Add Local…* and select
   `../FortKit`, then add the `FortKit` library product to the app target. (If
   the iOS app is itself an SPM target, depend on it by path instead — see the
   FortKit README.)
3. **Add the CarPlay entitlement and scene manifest.** Follow
   [`Info-CarPlay-notes.md`](./Info-CarPlay-notes.md): request the CarPlay
   entitlement from Apple, add the entitlement key to `Fort.entitlements`, and
   declare the `CPTemplateApplicationSceneSessionRoleApplication` scene (delegate
   `FortCarPlaySceneDelegate`) in `Info.plist`.
4. **Local networking.** For device builds, add the `NSLocalNetworkUsageDescription`
   string and any App Transport Security exception if you repoint `baseURL` off
   localhost (see the notes file).
5. **Run it.** In the iOS Simulator, use *Xcode ▸ I/O ▸ External Displays ▸
   CarPlay* to open the CarPlay window. The entitlement must be present in the
   build for the CarPlay scene to attach.

## Pointing at a different host

`FortClient` defaults to `http://127.0.0.1:4087`. The documented control-only
default is `:4091`. Set `client.baseURL` to repoint it — on a real device,
localhost is the phone itself, so you'll typically point it at the Mac/host
running the control plane.
