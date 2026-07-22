# Fort — iOS client

The iOS control-plane surface for Fort. Its thumb-reachable navigation is
**Deck, Direction, Projects, Today, More**. More keeps Playbooks, Crew, Week,
the inspectable Feed, and Settings available without crowding the primary bar.
It imports [`../FortKit`](../FortKit) and does **not** redefine the wire models
or client.

- **Platform:** iOS 16+ (matches FortKit's deployment floor)
- **Depends on:** `../FortKit` (local Swift Package; Foundation only)
- **Talks to:** Fort's control plane, default `http://127.0.0.1:4087`
  (configurable in-app via More ▸ Settings)

## Files

| File | Role |
|------|------|
| `FortApp.swift` | `@main` app entry; holds one `FortClient` as `@StateObject`, injects it into the environment, and provides the `SettingsView` host editor. |
| `BoardView.swift` | Command Deck: polls board, backlog, machines, metrics, and playbooks; owns the five-item navigation, route-preview handoff, secondary views, and `RunDetailView`. |
| `GatesView.swift` | Legacy standalone sign-off list retained for source compatibility; the active mobile inbox is in the Deck. |
| `FeedView.swift` | Activity sheet from More: consumes `client.events(since:)` with resume-on-reconnect. Also hosts shared helpers `EventRow`, `errorText`, and `ContentUnavailableCompat`. |

These are **source-only** — there is no `.xcodeproj` here. Add them to an Xcode
app target as described below.

## Adding it as an Xcode target

1. **Create the app target.** In Xcode: *File ▸ New ▸ Project… ▸ iOS ▸ App*.
   - Interface: **SwiftUI**, Language: **Swift**, Lifecycle: **SwiftUI App**.
   - Set the deployment target to **iOS 16.0**.
   - Because these files already provide a `@main` `FortApp`, **delete the
     template's generated `…App.swift` and `ContentView.swift`** so there's
     exactly one `@main`.

2. **Add these sources.** Drag `FortApp.swift`, `BoardView.swift`,
   `GatesView.swift`, and `FeedView.swift` into the target (*Add Files…*,
   "Copy items if needed" **off** so they stay in-tree), and confirm target
   membership.

3. **Add FortKit as a local package.** *File ▸ Add Package Dependencies… ▸
   Add Local…* and choose `../FortKit` (`ui/apple/FortKit`). Then in the app
   target's *General ▸ Frameworks, Libraries, and Embedded Content*, add the
   **FortKit** library product. Each Swift file already does `import FortKit`.

4. **Build & run** on the simulator or a device (see network note below).

> Prefer SwiftPM-only? You can instead make this a package target that depends
> on FortKit by path (`.package(path: "../FortKit")`) and generate the app
> shell with Xcode, but an Xcode app target is the simplest path to a runnable
> `.app`.

## Info.plist / entitlements

### App Transport Security (required for HTTP)

Fort's control plane is served over **plain HTTP** (`http://127.0.0.1:4087`).
iOS ATS blocks cleartext HTTP by default, so the app **will not connect** until
you add an ATS exception. Choose the narrowest one that fits your setup:

- **Simulator against `127.0.0.1` only** — allow local networking:

  ```xml
  <key>NSAppTransportSecurity</key>
  <dict>
    <key>NSAllowsLocalNetworking</key>
    <true/>
  </dict>
  ```

  `NSAllowsLocalNetworking` covers `localhost`, `*.local`, and link-local
  addresses. It does **not** cover an arbitrary LAN IP like `192.168.x.x`.

- **A device pointing at a Mac's LAN IP** — add a per-domain exception for that
  host (replace the example with your Mac's address; note dotted IPs work as
  the key here):

  ```xml
  <key>NSAppTransportSecurity</key>
  <dict>
    <key>NSExceptionDomains</key>
    <dict>
      <key>192.168.1.50</key>
      <dict>
        <key>NSExceptionAllowsInsecureHTTPLoads</key>
        <true/>
        <key>NSIncludesSubdomains</key>
        <false/>
      </dict>
    </dict>
  </dict>
  ```

Do **not** ship `NSAllowsArbitraryLoads` to the App Store without
justification. For local dev these local-scoped exceptions are the right call.

### Local Network permission (device, iOS 14+)

Reaching another host on the local network (e.g. your Mac from an iPhone)
triggers the iOS **Local Network** privacy prompt. Add a usage string so the
prompt is not rejected:

```xml
<key>NSLocalNetworkUsageDescription</key>
<string>Fort connects to your Mac's control plane on the local network.</string>
```

(The simulator talking to `127.0.0.1` is in-process and does not prompt.)

### Entitlements

No special **entitlements** are required for the base app — it makes outbound
HTTP/SSE requests only (no background modes, push, or App Groups needed for
this scaffold). Standard outbound networking works with the default sandbox.
If you later add a watch/CarPlay extension that shares config, an App Group
entitlement would be the place to start.

## How it behaves

- **Polling + streaming together.** The Deck polls every ~3s for a reliable
  snapshot; Activity holds the SSE stream open while its sheet is visible and
  resumes from the last-seen id on reconnect.
- **Control-only mode.** When `Summary.execution == false`, the Deck shows a
  banner, and any gate decision returns `false` (HTTP 409) from
  `decideGate` — the Deck surfaces a "no execution plane" alert instead of
  treating it as an error.
- **Configurable host.** More ▸ Settings edits the URL; `FortClient.baseURL` is
  `@Published`, so the Deck's keyed task restarts against the new host.
