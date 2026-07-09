# Fort for Mac (spec 032) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A native macOS app that both **runs** Fort (a `fort service` launchd manager, Go-testable) and **drives** it (a windowed SwiftUI Define/Ready/In-progress view on FortKit), plus a signed/notarized DMG build path (notarization is the operator's step).

**Architecture:** The daemon-lifecycle logic lives in Go as `fort service install|start|stop|restart|status|uninstall` (fully unit-testable against a temp LaunchAgents dir — no real `launchctl load` in CI). The SwiftUI app shells out to that subcommand via a FortKit `ServiceController`, and mirrors the 031 dashboard over the existing HTTP/SSE contract using the existing `FortClient`. The menu-bar app is retained; a new window scene is added. What CAN'T run headlessly (signing, notarization, GUI interaction) is compile-verified via `xcodebuild … CODE_SIGNING_ALLOWED=NO` and documented in a runbook.

**Tech Stack:** Go 1.26 (`fort service`, launchd plist); Swift/SwiftUI on the existing FortKit package + XcodeGen `project.yml`; `xcodebuild` (Xcode 26 available) for compile checks.

**Scope note (v1, honest):** in-app Google sign-in for the 028 gateway is scaffolded as a `GatewayAccount` model + a settings field for a gateway base URL + a machine picker, but the full ASWebAuthenticationSession OAuth flow is DEFERRED to a documented follow-on (it needs the deployed gateway from 028); local + mesh machines work fully. The native view mirrors 031 as a **read + act** surface (Define compose, Ready start, In-progress with activity, gate approve/reject, run drawer) — building on FortClient's existing calls; anything FortClient doesn't yet expose is added there.

---

## File Structure

- `cmd/fort/service.go` (+ `service_test.go`) — the `fort service` subcommand + launchd plist writer.
- `cmd/fort/service_darwin.go` / `service_other.go` — `launchctl` invocation behind a build tag (tests target the pure plist/path logic cross-platform).
- `cmd/fort/main.go` — `case "service"` + usage.
- `ui/apple/FortKit/Sources/FortKit/ServiceController.swift` — shells out to `fort service`, parses status.
- `ui/apple/FortKit/Sources/FortKit/GatewayAccount.swift` — gateway base URL + selected machine (Codable, persisted).
- `ui/apple/FortKit/Sources/FortKit/FortClient.swift` — extend with any missing dashboard calls (backlog list/dispatch, chat with title+body, breakdown, run detail) if absent.
- `ui/apple/macOS/FortWindow.swift` — the windowed dashboard (Define/Ready/In-progress + sidebar + service controls).
- `ui/apple/macOS/FortMacApp.swift` — add a `Window`/`WindowGroup` scene alongside the retained `MenuBarExtra`.
- `ui/apple/project.yml` — ensure the FortMac target includes the new sources (likely already globs `macOS/`).
- `Makefile` — `make mac-dmg` (archive → export → notarize → DMG; guarded).
- `docs/notes/mac-app.md` — build/sign/notarize/distribute runbook.

---

### Task 1: `fort service` (Go, TDD)

**Files:**
- Create: `cmd/fort/service.go`, `cmd/fort/service_test.go`
- Modify: `cmd/fort/main.go` (switch + usage)

- [ ] **Step 1.1: Write the failing test** — create `cmd/fort/service_test.go`:

```go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlistContentsAndPath(t *testing.T) {
	sc := serviceConfig{
		Label:   "io.tobsai.fort",
		BinPath: "/opt/homebrew/bin/fort",
		Args:    []string{"serve"},
		Addr:    "127.0.0.1:4087",
		DBPath:  "/Users/x/.fort-native/fort.db",
		LogDir:  "/Users/x/Library/Logs/Fort",
	}
	got := renderPlist(sc)
	for _, want := range []string{
		"<key>Label</key>", "<string>io.tobsai.fort</string>",
		"<string>/opt/homebrew/bin/fort</string>", "<string>serve</string>",
		"<key>FORT_ADDR</key>", "<string>127.0.0.1:4087</string>",
		"<key>RunAtLoad</key>", "<key>KeepAlive</key>",
		"<key>StandardOutPath</key>",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("plist missing %q\n%s", want, got)
		}
	}
	if p := plistPath("/Users/x", sc.Label); p != "/Users/x/Library/LaunchAgents/io.tobsai.fort.plist" {
		t.Errorf("plistPath = %q", p)
	}
}

func TestInstallWritesPlistUninstallRemoves(t *testing.T) {
	home := t.TempDir()
	sc := serviceConfig{Label: "io.tobsai.fort.test", BinPath: "/bin/echo", Args: []string{"serve"}}
	if err := writePlist(home, sc); err != nil {
		t.Fatal(err)
	}
	p := plistPath(home, sc.Label)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("plist not written: %v", err)
	}
	// idempotent rewrite
	if err := writePlist(home, sc); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if err := removePlist(home, sc.Label); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("plist not removed")
	}
	// remove is idempotent (no error when already gone)
	if err := removePlist(home, sc.Label); err != nil {
		t.Fatalf("second remove: %v", err)
	}
	_ = filepath.Join // keep import if unused after edits
}
```

- [ ] **Step 1.2: Run — expect FAIL** (`serviceConfig`, `renderPlist`, `plistPath`, `writePlist`, `removePlist` undefined).

Run: `go test ./cmd/fort/ -run 'TestPlist|TestInstall'`

- [ ] **Step 1.3: Implement `cmd/fort/service.go`** — the pure, testable core (plist render + file I/O), plus the `cmdService` dispatcher. Complete:

```go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// serviceConfig is the launchd user-agent definition for the Fort daemon.
type serviceConfig struct {
	Label   string
	BinPath string
	Args    []string
	Addr    string
	DBPath  string
	LogDir  string
}

func plistPath(home, label string) string {
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

func renderPlist(sc serviceConfig) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n<dict>\n")
	b.WriteString("  <key>Label</key>\n  <string>" + xmlEscape(sc.Label) + "</string>\n")
	b.WriteString("  <key>ProgramArguments</key>\n  <array>\n")
	b.WriteString("    <string>" + xmlEscape(sc.BinPath) + "</string>\n")
	for _, a := range sc.Args {
		b.WriteString("    <string>" + xmlEscape(a) + "</string>\n")
	}
	b.WriteString("  </array>\n")
	// Environment
	b.WriteString("  <key>EnvironmentVariables</key>\n  <dict>\n")
	if sc.Addr != "" {
		b.WriteString("    <key>FORT_ADDR</key>\n    <string>" + xmlEscape(sc.Addr) + "</string>\n")
	}
	if sc.DBPath != "" {
		b.WriteString("    <key>FORT_DB</key>\n    <string>" + xmlEscape(sc.DBPath) + "</string>\n")
	}
	b.WriteString("  </dict>\n")
	b.WriteString("  <key>RunAtLoad</key>\n  <true/>\n")
	b.WriteString("  <key>KeepAlive</key>\n  <true/>\n")
	if sc.LogDir != "" {
		b.WriteString("  <key>StandardOutPath</key>\n  <string>" + xmlEscape(filepath.Join(sc.LogDir, "fort.out.log")) + "</string>\n")
		b.WriteString("  <key>StandardErrorPath</key>\n  <string>" + xmlEscape(filepath.Join(sc.LogDir, "fort.err.log")) + "</string>\n")
	}
	b.WriteString("</dict>\n</plist>\n")
	return b.String()
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func writePlist(home string, sc serviceConfig) error {
	dir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if sc.LogDir != "" {
		_ = os.MkdirAll(sc.LogDir, 0o755)
	}
	return os.WriteFile(plistPath(home, sc.Label), []byte(renderPlist(sc)), 0o644)
}

func removePlist(home, label string) error {
	err := os.Remove(plistPath(home, label))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
```

- [ ] **Step 1.4: Run — expect PASS**: `go test ./cmd/fort/ -run 'TestPlist|TestInstall' -v`.

- [ ] **Step 1.5: Add the platform dispatcher** — `cmd/fort/service.go` gains `cmdService(args []string) error` handling `install|start|stop|restart|status|uninstall`, building the `serviceConfig` from `config.Load(os.Getenv)` (Label default `io.tobsai.fort`, BinPath from `os.Executable()`, Addr from `cfg.Addr`, DBPath from `cfg.DBPath`, LogDir `~/Library/Logs/Fort`), calling `writePlist`/`removePlist`, then invoking `launchctl` for the actual load/unload/kickstart. Put the `launchctl` calls behind `runtime.GOOS == "darwin"` guards so a non-darwin build just reports "unsupported". Use `exec.Command("launchctl", "bootstrap"|"bootout"|"kickstart"|"print", ...)`:
  - install: writePlist + `launchctl bootstrap gui/$UID <plist>` (ignore "already bootstrapped").
  - start: `launchctl kickstart gui/$UID/<label>`.
  - stop: `launchctl bootout gui/$UID/<label>` (ignore not-loaded).
  - restart: `launchctl kickstart -k gui/$UID/<label>`.
  - status: `launchctl print gui/$UID/<label>` → parse "state = running" / pid; print running/stopped + the addr; fall back to an HTTP probe of `cfg.Addr`/api/summary.
  - uninstall: stop + removePlist.
  Print clear success lines. `$UID` via `os.Getuid()`.

- [ ] **Step 1.6: Wire the command** — in `cmd/fort/main.go` switch, add `case "service": err = cmdService(os.Args[2:])`, and usage lines:

```
  fort service install             install + start the launchd user agent
  fort service start|stop|restart  control the running daemon
  fort service status              report running/stopped + address
  fort service uninstall           stop + remove the launchd agent
```

- [ ] **Step 1.7: Full gates + commit**

```bash
go test ./... -count=1 && go vet ./cmd/fort/ && go build ./...
git add cmd/fort/service.go cmd/fort/service_test.go cmd/fort/main.go
git commit -m "feat(cmd): fort service — launchd daemon lifecycle manager (spec 032)"
```

---

### Task 2: FortKit `ServiceController` + `GatewayAccount`, and any missing `FortClient` calls

**Files:**
- Create: `ui/apple/FortKit/Sources/FortKit/ServiceController.swift`, `GatewayAccount.swift`
- Modify: `ui/apple/FortKit/Sources/FortKit/FortClient.swift` (add missing dashboard calls)

- [ ] **Step 2.1: Read the existing FortKit** — `FortClient.swift` + `Models.swift`. Enumerate what it already exposes (Summary, gates, chat?). The dashboard needs: `summary()`, `board()`/runs, `backlog()` list, `dispatchBacklog(id)`, `chat(text)`, `breakdown(text)`, `runDetail(id)`, `gate(run,node,decision)`. Add ONLY the missing ones, following the file's existing async/URLSession style and `Models.swift` decodable types (add `BacklogItem`, `RunSummary.body`, `RunDetail`, `Event.nodeId`, tool/subagent handling to match the Go wire contract from specs 027/029/030/031).

- [ ] **Step 2.2: `ServiceController.swift`** — an `@MainActor ObservableObject` that shells out to the bundled/`PATH` `fort` binary:

```swift
import Foundation

@MainActor
public final class ServiceController: ObservableObject {
    public struct Status: Sendable { public var running: Bool; public var detail: String }
    @Published public private(set) var status = Status(running: false, detail: "unknown")
    public var fortBinaryURL: URL   // bundled in the app; falls back to /opt/homebrew/bin/fort

    public init(fortBinaryURL: URL) { self.fortBinaryURL = fortBinaryURL }

    @discardableResult
    public func run(_ args: [String]) async -> (Int32, String) {
        await withCheckedContinuation { cont in
            let p = Process(); p.executableURL = fortBinaryURL; p.arguments = args
            let pipe = Pipe(); p.standardOutput = pipe; p.standardError = pipe
            do { try p.run() } catch { cont.resume(returning: (-1, "\(error)")); return }
            p.terminationHandler = { proc in
                let d = pipe.fileHandleForReading.readDataToEndOfFile()
                cont.resume(returning: (proc.terminationStatus, String(decoding: d, as: UTF8.self)))
            }
        }
    }
    public func install() async { _ = await run(["service","install"]); await refresh() }
    public func start()   async { _ = await run(["service","start"]);   await refresh() }
    public func stop()    async { _ = await run(["service","stop"]);    await refresh() }
    public func restart() async { _ = await run(["service","restart"]); await refresh() }
    public func uninstall() async { _ = await run(["service","uninstall"]); await refresh() }
    public func refresh() async {
        let (code, out) = await run(["service","status"])
        status = Status(running: code == 0 && out.contains("running"), detail: out.trimmingCharacters(in: .whitespacesAndNewlines))
    }
}
```

- [ ] **Step 2.3: `GatewayAccount.swift`** — a Codable model persisted in UserDefaults holding `gatewayURL: URL?` and `selectedMachineID: String?`, with a note that OAuth sign-in is a deferred follow-on; for v1 it just lets the app point FortClient at a gateway base URL (the tunnel proxy) or a local/mesh host.

- [ ] **Step 2.4: Compile-check FortKit**

```bash
cd ui/apple/FortKit && swift build 2>&1 | tail -20
```

Expected: build succeeds (FortKit is a plain SwiftPM package — no signing needed).

- [ ] **Step 2.5: Commit**

```bash
git add ui/apple/FortKit/Sources/FortKit/
git commit -m "feat(fortkit): ServiceController + GatewayAccount + dashboard client calls (spec 032)"
```

---

### Task 3: the windowed app + packaging + docs

**Files:**
- Create: `ui/apple/macOS/FortWindow.swift`
- Modify: `ui/apple/macOS/FortMacApp.swift` (add a Window scene), `ui/apple/project.yml` (if needed), `Makefile`, `docs/notes/mac-app.md`

- [ ] **Step 3.1: `FortWindow.swift`** — a SwiftUI `NavigationSplitView`: sidebar = machines (local + mesh from `/api/machines`, gateway later) + a "Service" section with install/start/stop/restart buttons bound to `ServiceController` and a running/stopped indicator; detail = the dashboard mirror (Define compose with a multiline `TextEditor` first-line=title, Ready list with Start buttons, In-progress list with status + tool/subagent activity rows, gate approve/reject) driven by `FortClient` on a refresh timer + the SSE feed if FortClient exposes it. Keep it a faithful but native mirror of 031; reuse `FortClient`/`Models`.

- [ ] **Step 3.2: Add the Window scene** — in `FortMacApp.swift`, keep `MenuBarExtra` and add:

```swift
        Window("Fort", id: "main") {
            FortWindow()
                .environmentObject(client)
                .environmentObject(service)
                .frame(minWidth: 720, minHeight: 480)
        }
```

with a `@StateObject private var service = ServiceController(fortBinaryURL: Self.bundledFort())` and a `bundledFort()` helper that returns `Bundle.main.url(forResource:"fort", withExtension:nil, subdirectory:"…")` if bundled, else `/opt/homebrew/bin/fort`.

- [ ] **Step 3.3: Compile-check the Mac target** (no signing):

```bash
cd ui/apple && xcodegen generate
xcodebuild -project Fort.xcodeproj -scheme FortMac -destination 'platform=macOS' \
  CODE_SIGNING_ALLOWED=NO build 2>&1 | tail -25
```

Expected: `** BUILD SUCCEEDED **`. Fix compile errors until it does. (If `xcodegen` isn't installed, `brew install xcodegen` or note the project must be regenerated by the operator; the sources must still compile — verify what you can with `swiftc -typecheck` against FortKit as a fallback and NOTE it.)

- [ ] **Step 3.4: `make mac-dmg` + docs** — add a `mac-dmg` Makefile target (archive → `-exportArchive` with `ExportOptions.plist` → `create-dmg` or `hdiutil` → `xcrun notarytool submit` → `xcrun stapler staple`), each step guarded with a comment that it needs the operator's Apple ID/team. Write `docs/notes/mac-app.md`: prerequisites (Xcode, team T3JB5MYZ93, an app-specific password / notarytool keychain profile), the exact commands, and where the bundled `fort` binary comes from (`make build` → copy into `Contents/Resources` — add that copy step to the archive flow or document it). Be explicit that notarization is the operator's step and cannot run in CI.

- [ ] **Step 3.5: Commit**

```bash
git add ui/apple/macOS/ ui/apple/project.yml Makefile docs/notes/mac-app.md
git commit -m "feat(mac): windowed Fort.app — service controls + native dashboard; DMG runbook (spec 032)"
```

---

## Self-review

**Spec coverage:** the app runs the daemon (`fort service` install/start/stop/restart/status/uninstall — T1, Go-tested; `ServiceController` shells to it — T2; sidebar controls — T3); native Define/Ready/In-progress mirror of 031 over the existing HTTP/SSE contract (T3 on FortClient — T2); machines sidebar local+mesh (T3); menu-bar retained + window added (T3.2); markdown/subagent rendering carried via the client models (T2.1); notarized DMG runbook (T3.4). Gateway sign-in: scaffolded (`GatewayAccount`) with the full OAuth flow explicitly deferred (documented) — the honest v1 boundary, since it needs the deployed 028 gateway.
**Placeholder scan:** T1 code is complete; T2/T3 Swift specifies exact types/behaviors with a compile gate rather than full listings for the large view code (the SwiftUI view mirrors 031's known layout and must compile — the gate is `xcodebuild BUILD SUCCEEDED`). Deferred items are named explicitly, not hidden as TBDs.
**Type consistency:** `serviceConfig`/`renderPlist`/`writePlist`/`removePlist`/`plistPath` names match across T1 test+impl; `ServiceController.run(["service", …])` matches the Go subcommand verbs; FortClient additions match the Go wire contract (RunSummary.body, Event.node_id, tool/subagent) from 027/029/030/031.
