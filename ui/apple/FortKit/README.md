# FortKit

FortKit is the shared Swift Package that **every Fort Apple surface imports** —
the iOS app, the macOS app, the watchOS complication, and the CarPlay scene. It
is the single, authoritative client for Fort's control-plane HTTP/SSE contract,
so no surface hand-rolls URLs, JSON shapes, or SSE parsing.

- **Platforms:** iOS 16+, macOS 13+, watchOS 9+
- **Tools:** swift-tools 5.9
- **Dependencies:** no external packages (Foundation + CryptoKit)

The wire models mirror the authoritative Go source at
[`ui/contract.go`](../../contract.go) exactly — field names, snake_case JSON
keys, and optionality. If the Go contract changes, update `Models.swift` to
match.

## What's inside

- **`Models.swift`** — the Codable, Sendable wire types: `Summary`, `Board`,
  `RunSummary`, `GateItem`, `NodeSummary`, `RunDetail`, `Event`, `ChatResult`,
  `ActionResult`, `ChatRequest`, `GateDecision`, `OpenClawMessage`. Each declares
  explicit `CodingKeys` for its snake_case JSON keys.
- **`FortClient.swift`** — `FortClient`, an `ObservableObject` that performs the
  reads, commands, and the SSE live feed over either a direct host or the native
  encrypted gateway transport.
- **`GatewayRelay.swift`** — authenticated gateway machine discovery and
  fetch/SSE transport over sealed relay frames.
- **`SecureRelay.swift`** — Fort's native Noise IK / ChaCha20-Poly1305 mirror,
  byte-checked against the Go daemon vector.
- **`GatewayAccount.swift`** — persisted gateway, native session, selected
  machine, and TOFU public-key pins.

## Adding it to a surface

In another package's `Package.swift`, depend on FortKit by path:

```swift
dependencies: [
    .package(path: "../FortKit"),
],
targets: [
    .target(
        name: "FortiOS",
        dependencies: [
            .product(name: "FortKit", package: "FortKit"),
        ]
    ),
]
```

In an Xcode app project, add the FortKit package (File ▸ Add Package
Dependencies ▸ Add Local…) and link the `FortKit` library to your app target.

Then:

```swift
import FortKit
```

## Using the client

`FortClient` is an `ObservableObject`, so hold it as `@StateObject` at the app
root and inject it via the environment:

```swift
import SwiftUI
import FortKit

@main
struct FortApp: App {
    @StateObject private var client = FortClient() // http://127.0.0.1:4087

    var body: some Scene {
        WindowGroup {
            ContentView().environmentObject(client)
        }
    }
}
```

Point it at a different host by setting `baseURL` (it's `@Published`):

```swift
client.baseURL = URL(string: "http://127.0.0.1:4091")!
```

For remote access, discover a `GatewayMachine`, verify its displayed
fingerprint against the host, persist its public key in `GatewayAccount`, then
call `client.useGateway(account:machine:)`. The same typed methods below will
use a fresh pinned Noise tunnel per operation; callers do not build relay
frames themselves.

### Reads

```swift
let summary = try await client.summary()
let board   = try await client.board()
let gates   = try await client.gates()
let detail  = try await client.runDetail(runID)
```

### Commands

```swift
let result = try await client.chat("ship the release notes")
let inbound = try await client.openclaw(from: "toby", text: "status?")
```

### Deciding a gate — control-only mode

`decideGate` returns **`false`** when the server replies **HTTP 409** (no
execution plane attached — control-only mode) and **`true`** on success. It does
**not** throw on 409, so surfaces handle it gracefully:

```swift
let applied = try await client.decideGate(
    run: gate.runID,
    node: gate.nodeID,
    decision: "approve",   // or "reject"
    edit: nil
)
if !applied {
    // control-only: show "no execution plane"
}
```

`Summary.execution == false` is the same signal at a glance: chat only boards a
queued task and gate actions will 409.

### Live feed (SSE)

`events(since:)` is an `AsyncThrowingStream<Event, Error>` parsed from
`GET /api/events`. Iterate it in a `Task`; cancelling the task closes the stream.

```swift
let feed = Task {
    do {
        for try await event in client.events(since: lastEventID) {
            // apply event to your view state
        }
    } catch {
        // transport/parse failure — reconnect with backoff, resuming from lastEventID
    }
}
// feed.cancel() to stop
```

Pass the highest `Event.id` you've seen as `since` when reconnecting to replay
from that point.
