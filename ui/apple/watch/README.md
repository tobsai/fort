# FortWatch

A watchOS surface for Fort's control plane: a glanceable summary of run counts,
the pending-gate inbox with a one-tap **Approve** on the first gate, and a
WidgetKit complication showing the pending-gate count on the watch face.

This is a **source-only scaffold** — the `.swift` files here are meant to be
dropped into an Xcode watchOS target. There is no `.xcodeproj` or `Package.swift`
in this folder; the watch app is an app target, not a package.

## Files

- **`FortWatchApp.swift`** — `@main App`. Owns the shared `FortClient` and
  injects it into the environment.
- **`GlanceView.swift`** — the app's single screen: summary counts + gate inbox
  with a one-tap Approve on the first pending gate.
- **`FortComplication.swift`** — a WidgetKit complication (`Widget` +
  `TimelineProvider`) for `accessoryCircular` / `accessoryInline`, showing the
  pending-gate count.

## Dependency: FortKit

Everything talks to Fort through **[`../FortKit`](../FortKit)**, the shared Swift
package. This surface does **not** redefine the wire models or the client — it
imports them:

```swift
import FortKit
```

`FortClient` defaults to `http://127.0.0.1:4087`. Point it elsewhere by setting
`client.baseURL`. Note that on a real watch `127.0.0.1` resolves to the watch
itself — for a paired-iPhone or LAN control plane, set `baseURL` to the reachable
host/IP, and see the App Transport Security note below.

## Adding it as an Xcode target

1. In your Fort Apple workspace/project, **File ▸ New ▸ Target… ▸ watchOS ▸ App**.
   Name it `FortWatch`. (For a standalone watch app, no companion iOS app is
   required on watchOS 9+.)
2. Delete the stub `ContentView.swift`/`App` file Xcode generates and **add the
   files in this folder** (`FortWatchApp.swift`, `GlanceView.swift`) to the watch
   app target. Right-click the group ▸ *Add Files to…*, or drag them in. Confirm
   **Target Membership** for each is the watch app.
3. **Add the FortKit package:** *File ▸ Add Package Dependencies… ▸ Add Local…*,
   select `ui/apple/FortKit`, and link the **FortKit** library product to the
   watch app target (General ▸ *Frameworks, Libraries, and Embedded Content*).

### Complication (widget extension)

The complication ships as a **watchOS Widget Extension**, a separate target:

1. **File ▸ New ▸ Target… ▸ watchOS ▸ Widget Extension**. Name it e.g.
   `FortWatchWidgets`. Uncheck "Include Configuration Intent" (this uses
   `StaticConfiguration`).
2. Add **`FortComplication.swift`** to that widget-extension target (not the app
   target).
3. Link **FortKit** to the widget-extension target too (same *Add Package
   Dependencies* flow; FortKit supports watchOS 9+).
4. Provide a `@main` entry for the extension. `FortComplication` is a single
   `Widget`; either mark it `@main` directly, or wrap it in a `WidgetBundle`:

   ```swift
   @main
   struct FortWatchWidgetBundle: WidgetBundle {
       var body: some Widget { FortComplication() }
   }
   ```

   (If you add the bundle, remove the `@main`-implied entry so there's exactly
   one `@main` in the extension.)

## Deployment target

Set **watchOS 9.0+** for both targets (FortKit's floor, and the floor for the
accessory widget families used here).

## Entitlements & Info.plist

`FortClient` makes plain-HTTP requests to a local/LAN control plane, so App
Transport Security must allow them. Add to the **watch app's** `Info.plist`
(and the widget extension's, since its `TimelineProvider` also fetches):

```xml
<key>NSAppTransportSecurity</key>
<dict>
    <!-- Local dev against http://127.0.0.1:4087 / :4091 -->
    <key>NSAllowsLocalNetworking</key>
    <true/>
</dict>
```

`NSAllowsLocalNetworking` covers `localhost`, `*.local`, and link-local/private
addresses without disabling ATS globally. If you point `baseURL` at a specific
non-local host over plain HTTP, add an `NSExceptionDomains` entry for that host
instead of using `NSAllowsArbitraryLoads`.

If the control plane is reachable only over the **local network** (e.g. a Mac on
the same Wi-Fi as the watch), also add a usage string so the local-network
permission prompt has copy:

```xml
<key>NSLocalNetworkUsageDescription</key>
<string>Fort connects to your Mac's control plane to show runs and gates.</string>
```

No other capabilities are required: the app uses no HealthKit, location,
background modes, or push. If you later add **complication push updates** or
background refresh, add the relevant background modes / `WKBackgroundModes` and a
push entitlement at that point — the current scaffold refreshes the timeline on
WidgetKit's schedule (every ~15 min) and on app foreground only.

### WKCompanionAppBundleIdentifier / bundle IDs

For a dependent (non-standalone) watch app, set the watch app's
`WKCompanionAppBundleIdentifier` to the iOS app's bundle ID and keep the widget
extension's bundle ID prefixed by the watch app's (e.g.
`io.mtree.fort.watch.widgets`). A standalone watchOS 9 app doesn't need a
companion, but the widget-extension bundle ID must still be prefixed by the watch
app's.

## Behavior notes

- **Control-only mode.** When `Summary.execution == false`, the summary shows a
  "Control-only — no execution plane" line. If you tap **Approve** in that mode,
  `client.decideGate` returns `false` (the server replies **HTTP 409**) and the
  screen shows a non-fatal notice ("No execution plane — can't approve here")
  rather than an error. The complication still shows the gate count with a
  "control-only" widget label.
- **First-gate approve only.** By design the glance only exposes Approve on the
  **first** pending gate — the constrained-surface affordance. Full triage
  (reject, edit, per-gate) lives on the iOS/macOS surfaces.
