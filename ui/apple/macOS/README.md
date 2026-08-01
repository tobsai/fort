# FortMac — macOS Command Deck

A windowed SwiftUI Command Deck plus `MenuBarExtra` for Fort's control plane.
The primary sidebar mirrors the web redesign's Command Deck, Conversations,
Today, Crew, and Playbooks destinations while retaining native service and
machine controls. New conversation is the single creation action, and Week is
reachable from Today. The
menu leads with **Needs you**, supports **Accept / Request changes**, and keeps
quick direction available. The menu-bar icon badges the pending sign-off count.

All Fort I/O goes through the shared **[FortKit](../FortKit)** Swift package —
this surface does **not** redefine the wire models or the HTTP/SSE client, it
imports them.

## Files

- **`FortMacApp.swift`** — `@main` `App` with a primary window and the
  `MenuBarExtra` scene. Owns the shared clients and badge state.
- **`FortWindow.swift`** — the native Command Deck, Playbooks pane, route-preview composer, and service sidebar.
- **`MenuContent.swift`** — the popover body: counts, gate rows, chat field,
  status footer, and the poll/action logic.

## Dependency

This surface depends on the local **FortKit** package at `../FortKit`
(`ui/apple/FortKit`). It has no other dependencies.

## Adding it as an Xcode target

There is no `.xcodeproj` checked in (source-only scaffold). To build and run:

1. **Create the app project.** In Xcode: *File ▸ New ▸ Project… ▸ macOS ▸ App*.
   - Product Name: `FortMac`
   - Interface: **SwiftUI**, Life Cycle: **SwiftUI App**, Language: **Swift**
   - Minimum Deployment: **macOS 13.0** (matches FortKit).
2. **Add these sources.** Delete the generated `ContentView.swift` and the
   generated `<Name>App.swift`, then add `FortMacApp.swift`, `FortWindow.swift`,
   and `MenuContent.swift` from this folder to the app target (drag them in, or
   *File ▸ Add Files…*). Keep "Copy items if needed" unchecked so they stay in
   place under version control.
3. **Add the FortKit package.** *File ▸ Add Package Dependencies… ▸ Add Local…*
   and select `ui/apple/FortKit`. Link the **FortKit** library product to the
   `FortMac` target (Target ▸ General ▸ Frameworks, Libraries, and Embedded
   Content).
4. **Build & run.** The app opens its window and appears in the menu bar. Point it at a non-default
   host by setting `client.baseURL` (default `http://127.0.0.1:4087`; the
   docs' control-only default is `http://127.0.0.1:4091`).

## Info.plist keys

- **`LSUIElement` = `NO`** (Boolean). FortMac is windowed and keeps its Dock
  icon; the `MenuBarExtra` remains available for quick attention handling.
- **`NSAppTransportSecurity`** — the control plane is plain **HTTP on
  localhost** (`http://127.0.0.1:…`), which App Transport Security blocks by
  default. Allow local networking:

  ```xml
  <key>NSAppTransportSecurity</key>
  <dict>
      <key>NSAllowsLocalNetworking</key>
      <true/>
  </dict>
  ```

  `NSAllowsLocalNetworking` permits cleartext to `localhost` / `127.0.0.1`
  without opening HTTP to the whole internet. Prefer it over
  `NSAllowsArbitraryLoads`.

## Entitlements (App Sandbox)

A distributed/notarized macOS app runs under the **App Sandbox**. The client
makes outbound HTTP/SSE calls, so it needs the outgoing-network entitlement:

- **`com.apple.security.app-sandbox` = `YES`**
- **`com.apple.security.network.client` = `YES`** — required for the
  `URLSession` reads, commands, and the SSE live feed to reach the control
  plane.

`.entitlements` snippet:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>com.apple.security.app-sandbox</key>
    <true/>
    <key>com.apple.security.network.client</key>
    <true/>
</dict>
</plist>
```

No server, file-access, or hardware entitlements are needed — the app only makes
outbound connections and holds no persistent state.

> Note: the sandbox does **not** exempt `localhost` from ATS. You still need the
> `NSAllowsLocalNetworking` ATS key above **and** `network.client` for the
> connection to succeed.

## Control-only mode

When `Summary.execution == false`, Fort has no execution plane attached:

- The header shows a **"Control-only"** badge.
- Chat only **boards a queued task** (`ChatResult.queued == true`); the app
  notes this in the footer.
- Gate decisions return **HTTP 409** — `FortClient.decideGate` returns `false`
  (it does *not* throw), and the app shows a non-fatal
  *"No execution plane — gate action unavailable."* notice.
