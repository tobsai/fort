# Spec 035 — Apple Command Deck Redesign

**Status:** approved-by-instruction (Toby: "Using the design document update the UI and application according to the designs and spec")
**Design source:** `design_handoff_fort_dashboard_redesign/` and the implemented web dashboard from spec 033.
**Supersedes:** spec 034's unimplemented WebKit-host proposal.

## Goal

Bring the iPhone and macOS control-plane clients into the web dashboard's
delegation model. The first surface answers "what needs me?", uses project
sigils and checkpoint-based progress, replaces machine vocabulary with human
words, and exposes the same project, roster, performance, Week, and Today
perspectives where the native form factor supports them.

## Approach

1. Update FortKit's additive wire models for the spec-033 API: timestamps,
   machine placement, checkpoint summaries, gate age and redirect notes; add
   metrics and backlog-reassignment client calls.
2. Add shared, deterministic presentation logic for project state, checkpoint
   captions, and the mirrored 5x5 FNV-1a/xorshift sigil grammar.
3. Replace the iPhone Board/Gates split with a dark, inbox-first Command Deck:
   needs-you cards, projects, crew, assignment composer, and native Projects,
   Performance, Week, and Today views. Keep Feed available as a secondary tab.
4. Replace the macOS window's Define/Ready/In-progress dashboard with a native
   multi-column Command Deck and sidebar navigation for Deck, Projects, Assign,
   Performance, Week, and Today. Retain service and machine controls in a
   utility section. Align the menu-bar popover vocabulary and attention order.

The clients continue to call Fort only through FortKit. Schedule views derive
honestly from current runs, gates, and Up-next work; recurring scheduler blocks
remain deferred because the API does not list them.

## Affected files

- `ui/apple/FortKit/Package.swift`
- `ui/apple/FortKit/Sources/FortKit/Models.swift`
- `ui/apple/FortKit/Sources/FortKit/FortClient.swift`
- `ui/apple/FortKit/Sources/FortKit/CommandDeck.swift` (new)
- `ui/apple/FortKit/Tests/FortKitTests/CommandDeckTests.swift` (new)
- `ui/apple/iOS/FortApp.swift`
- `ui/apple/iOS/BoardView.swift`
- `ui/apple/iOS/GatesView.swift`
- `ui/apple/macOS/FortWindow.swift`
- `ui/apple/macOS/MenuContent.swift`

## Test criteria

- FortKit tests decode a full spec-033 board/metrics payload and encode redirect
  notes and backlog patches.
- Sigils are deterministic, mirrored, and stable for a known project name.
- Project-state and checkpoint-caption tests cover needs-you, working,
  delivered, idle, and mixed checkpoint states.
- `swift run FortKitContractChecks` passes in FortKit.
- Generated Xcode project builds the iOS Simulator and macOS targets without
  code signing.
- `go test ./...` stays green.
- At iPhone width, primary decisions and assignment actions remain on-screen;
  the macOS window remains usable at its declared minimum size.

## Rollback

Revert the Apple-client commit. The server contract is unchanged and all new
wire fields remain additive.
