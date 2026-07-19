# 034 — Mac app hosts the web board (delegation dashboard parity)

**Status:** spec written on Toby's request (2026-07-19) — **awaiting approval before implementation.**
**Governed by:** [021-fort-native](021-fort-native.md) · aligns the Mac window with the 033 redesign ([033-dashboard-redesign](033-dashboard-redesign.md)) · extends the Mac app shell ([032-fort-for-mac](032-fort-for-mac.md)).

## Goal

Bring the spec-033 delegation dashboard (Deck / Projects / Assign / Performance /
Week / Today) to the Mac app by **hosting the daemon's own web board in the main
window** (`WKWebView` → `http://127.0.0.1:4087/`, honoring the configured
`FORT_ADDR`). One interface, maintained once: every future web-board change
lands in the Mac app automatically.

## Non-goals (v1 — YAGNI)

- **No native SwiftUI reimplementation of the 033 design.** That is the
  explicitly deferred alternative: FortKit models for `checkpoints` / gate
  `since` / `/api/metrics` / reject notes plus a full view rewrite adapted to
  macOS idioms. Own spec if the hosted page grates day-to-day.
- **No change to the menu-bar popover, watch complication, CarPlay, or iOS
  app.** Glanceable surfaces stay native over `/api/summary`—that is what they
  are good at.
- **No change to the daemon lifecycle features from 032.** Install/Start/Stop/
  Restart/logs stay native and visible even when the board can't load.
- **No new HTTP contract, no remote-gateway hosting.** The web view points at
  the local daemon only (the gateway portal already renders a remote snapshot).

## Approach

- New `BoardWebView` (SwiftUI `NSViewRepresentable` wrapping `WKWebView`) in
  `ui/apple/macOS/`, loading `http://<FORT_ADDR>/` (default `127.0.0.1:4087`).
  - Non-persistent website data store is **not** used — keep the default store
    so the board's `localStorage` (theme, selected view) survives relaunches.
  - Navigation policy: same-origin only; external links (`target=_blank`,
    other hosts) open in the default browser via `NSWorkspace`.
  - JS enabled; no script injection; no custom URL schemes.
- `FortWindow.swift`: the main window's dashboard content is replaced by
  `BoardWebView`. The 032 service-controls bar remains native (toolbar or
  header strip). When the daemon is unreachable, show the existing native
  status/empty state with the service controls (Start) instead of a white
  error page; auto-reload the web view once the health poll succeeds.
- Reload button + `Cmd-R` wired to `WKWebView.reload()`.
- The retired native board views stay in the tree this v1 (dead code flagged,
  removed in a follow-up once the hosted board has proven itself).

## Affected files

- Add: `ui/apple/macOS/BoardWebView.swift`
- Modify: `ui/apple/macOS/FortWindow.swift` (embed BoardWebView + offline
  fallback), `ui/apple/macOS/FortMacApp.swift` (menu/Cmd-R wiring)
- Modify (only if the sandbox/entitlements need the network-client key or
  WebKit linkage): `ui/apple/project.yml`, `ui/apple/Support/FortMac-Info.plist`
- No Go changes. No FortKit contract changes.

## Test criteria

- `make apple-build` green (all targets).
- Launch Fort.app with the daemon running → the window shows the 033 board
  (Deck default), theme toggle + view switching + drawer work, actions
  (Approve / Request changes… / Hand it off) hit the local API.
- Quit daemon → window falls back to native status + Start control; starting
  the service brings the board back without relaunching the app.
- External links from run bodies open in the default browser, not the window.
- Menu bar popover, watch, CarPlay, iOS unchanged (`/api/summary` only).

## Rollback

Revert the single implementation commit — the window returns to the native 032
dashboard. No data, schema, or API changes involved.
