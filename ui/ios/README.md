# Fort iOS shell (AO-037)

A minimal SwiftUI client pointed at `fort-core`. It shows live runs and lets you
approve/reject gates from your phone — the mobile surface Multica lacks.

## What it does
- `GET /api/board` every 2s → renders the **Runs** list and **Gate inbox**.
- **Approve/Reject** buttons → `POST /api/gate` → resumes/branches the paused flow.

## Build
This is source-only scaffold (no Xcode project committed, and Xcode isn't
available in the build environment):

1. Create an iOS App target in Xcode (SwiftUI lifecycle).
2. Add `FortApp.swift`.
3. Set `FortClient.base` to your fort-core address. For a device on your LAN use
   the host's IP (e.g. `http://192.168.1.20:4087`) and allow it under App
   Transport Security, or run fort-core behind TLS.
4. Run `fort serve` on the host, then build & run the app.

## Contract
The Swift `Codable` types mirror `ui/contract.go` (`Board`, `RunSummary`,
`GateItem`, `GateDecision`). For a live feed instead of polling, subscribe to the
`GET /api/events` SSE stream (see `docs/notes/event-contract.md`).
