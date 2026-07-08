# 032 — Fort for Mac (full windowed app that runs the daemon)

**Status:** design approved in brainstorm (Toby, 2026-07-08) — pending written-spec review.
**New capability — approved before implementation** (a native app that manages the daemon lifecycle).
**Governed by:** [021-fort-native](021-fort-native.md) · consumes the gateway ([028](028-remote-gateway.md)), mirrors the dashboard ([031](031-simplified-dashboard.md)), renders subagent events ([030](030-subagent-events.md)); extends the existing Apple clients (`ui/apple`).

## Goal
Make Fort feel like an app on the Mac: a native SwiftUI window that both **runs**
Fort (owns `fort serve` as a managed background service, replacing a hand-made
launchd plist) and **drives** it — a native Define/Ready/In-progress mirror of
the 031 dashboard over the same HTTP/SSE contract, plus machines from local,
mesh, and the 028 gateway.

## Non-goals (v1 — YAGNI)
- **No reimplementation of orchestration in Swift.** The app is a client + a
  process manager; all routing/execution stays in the Go daemon.
- **No Mac App Store submission in v1.** Ship as a signed + **notarized DMG**
  (direct download). MAS is a possible follow-on.
- **No bundled agent CLIs.** The app bundles the `fort` binary; the user still
  provides `claude`/`codex`/etc. and provider keys (as today).
- **No new HTTP contract.** Reuses the daemon's existing API/SSE (and 028's
  gateway session for remote machines). FortKit already speaks it.
- **No replacement of the web dashboard.** The Mac app mirrors 031; the served
  web board remains the canonical UI the app also embeds/links.

## Approach

### Two responsibilities, cleanly separated
1. **Daemon lifecycle (`ServiceController` in FortKit/app):**
   - Bundles the `fort` binary in the app (`Contents/Resources`).
   - Installs/loads a **launchd user agent** (`io.tobsai.fort`) pointing at the
     bundled binary — **replacing the hand-authored plist** currently in use.
   - Controls: **Install · Start · Stop · Restart · Reveal logs · Uninstall**;
     shows live status (running/stopped, port, version) via a status poll.
   - Reads/writes the daemon's env the same way today's setup does (`FORT_ADDR`,
     `FORT_DB`, mesh/relay env), surfaced as app settings.
2. **Native dashboard (SwiftUI on FortKit):**
   - **Sidebar of machines**: local daemon, mesh peers (`/api/machines`, 024),
     and **gateway machines** (028) once a gateway account is signed in.
   - **Main view** mirrors 031: **Define** (multiline markdown compose + agent/
     machine pickers + Run / Add to Ready / Break down), **Ready** (start
     items), **In progress** (live runs with `tool`/`subagent` activity from 030,
     inline gate approvals). Markdown bodies render natively (029 semantics: a
     Swift safe-subset renderer or `AttributedString(markdown:)` with links
     sanitized to http(s)).
   - A **run detail** pane mirrors the 027 drawer (steps + per-step live log).
   - **Menu-bar item retained** (from the existing macOS client): quick status +
     open-window + start/stop.

### Gateway sign-in (028)
The app gains a **Google sign-in** (ASWebAuthenticationSession) that obtains a
028 gateway session; gateway machines then appear in the sidebar and are driven
through the gateway base URL. Local/mesh machines are driven directly (as today).
Purely local use needs no sign-in.

### Packaging
- Built via the existing `ui/apple` Xcode project (`make apple-build` gains a Mac
  DMG target). Signed with the existing team **T3JB5MYZ93**; **notarized**;
  output a `.dmg`. The bundled `fort` binary is the release build for the host
  arch(s) (universal if feasible).
- The app is self-contained: first launch offers **"Install & start Fort
  service"**, which lays down the launchd agent and starts the daemon.

### Seams
The Go module is untouched except (optionally) a `fort service
install|start|stop|status|uninstall` subcommand the app shells out to (keeps
launchd logic testable in Go rather than Swift). The app talks only the existing
HTTP/SSE contract + 028 gateway session; no new server endpoints.

### Failure handling
- Bundled binary missing/incompatible arch → the app surfaces a clear error and
  offers to re-install.
- Daemon already running (e.g. a Homebrew/launchd instance on the same port) →
  detect the port in use, show status, and don't double-start; offer to adopt or
  restart.
- Gateway offline / not signed in → gateway machines show offline; local + mesh
  keep working.

## Architecture (respects the seams)
- **`cmd/fort/service.go`** (Go, optional but recommended) — `fort service
  install|start|stop|restart|status|uninstall` managing the launchd user agent
  (testable in Go; the app shells out to it).
- **`ui/apple/macOS/**`** — the SwiftUI windowed app (Define/Ready/In-progress,
  run detail, machines sidebar, settings, gateway sign-in), building on FortKit.
- **`ui/apple/FortKit/Sources/**`** — `ServiceController` (launchd/bundled-binary
  management) + a Swift markdown safe-subset renderer + the 030 event rendering;
  `GatewayAccount` reused from 028.
- **`ui/apple/Support/FortMac-Info.plist`**, `ExportOptions.plist`, `project.yml`
  — Mac app target, notarization config.
- **`Makefile`** — `make mac-dmg` (archive → export → notarize → DMG).
- **`docs/notes/testflight.md`** / a new `docs/notes/mac-app.md` — build + sign +
  notarize + distribute runbook.

## Decisions
- **D1 — full windowed app, not menu-bar-only.** Chosen by Toby; the menu-bar
  item is retained as a companion, not the whole product.
- **D2 — the app owns the daemon.** It installs/manages the launchd agent and
  bundles the binary, replacing the hand-made plist — "Fort is just an app."
- **D3 — client of the existing contract.** No orchestration in Swift; mirrors
  031 over the same API/SSE, so web and Mac stay in lockstep.
- **D4 — launchd logic in Go (`fort service`).** Testable, reused by any
  installer; the app shells out rather than embedding plist logic in Swift.
- **D5 — notarized DMG, not MAS (v1).** Fastest path to a distributable app;
  MAS/sandboxing deferred.
- **D6 — gateway optional.** Local/mesh use needs no Google sign-in; the gateway
  (028) only lights up remote machines.

## Affected files
- `cmd/fort/service.go` (new) + `cmd/fort/main.go` usage — `fort service …`.
- `ui/apple/macOS/**` (new/expanded) — the windowed app.
- `ui/apple/FortKit/Sources/**` — `ServiceController`, Swift markdown renderer,
  030 event rendering, `GatewayAccount` (028).
- `ui/apple/Support/*`, `ui/apple/project.yml`, `ui/apple/ExportOptions.plist` —
  Mac target + notarization.
- `Makefile` — `make mac-dmg`.
- `docs/notes/mac-app.md` (new) — build/sign/notarize/distribute.

## Test criteria
- `fort service` (Go): `install` writes a launchd plist pointing at the given
  binary; `status` reports running/stopped + port; `stop`/`start`/`uninstall`
  are idempotent; unit-tested against a temp `LaunchAgents` dir (no real load in
  CI). `go test ./cmd/... -race` green.
- `ServiceController` (Swift): start→status→stop drives `fort service` and
  reflects state (a local integration check with `FORT_FAKE=1`).
- Dashboard parity: the app's Define posts title/body like 031 (first line =
  title); Start dispatches a Ready item; a running run shows a `subagent`
  activity row (030); a gate approve posts `/api/gate`.
- Markdown: the Swift renderer passes the same injection corpus as 029 (no
  executable content; http(s)-only links).
- Gateway: signed-in, a 028 gateway machine appears in the sidebar and its board
  loads through the gateway; signed-out, only local/mesh appear.
- Build: `make apple-build` compiles all targets; `make mac-dmg` produces a
  signed, notarized DMG (manual/CI-gated step, documented).

## Rollback
The Go module change (`fort service`) is additive and revertible. The Mac app is
a new build target — not shipping it changes nothing about the daemon or other
clients. Users can continue with the hand-made launchd plist; the app's installer
is opt-in and reversible via **Uninstall** (removes the agent it created).
