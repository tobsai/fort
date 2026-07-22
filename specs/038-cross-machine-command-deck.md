# Spec 038 — Cross-machine Command Deck completion

**Status:** approved-by-instruction (Toby, 2026-07-22: “I want it to operate
across machines,” plus Playbooks, historical-attention, and Mac navigation
repairs).
**Governed by:** [022-multi-machine-orchestration](022-multi-machine-orchestration.md),
[035-apple-command-deck-redesign](035-apple-command-deck-redesign.md),
[036-playbooks](036-playbooks.md), and
[037-gateway-command-deck](037-gateway-command-deck.md).

## Goal

Make one Command Deck operate the configured Fort mesh rather than behaving
like a machine-local task launcher. Unpinned direct assignments already use the
hub's deterministic machine placer; compiled Playbook stages must use that same
placer. Complete the remote gateway's Playbooks surface, keep historical
failures truthful without presenting all of them as current human attention,
and restore working macOS sidebar navigation.

## Approach

1. Add an optional deterministic `Placer` seam to `graph.Executor`. Before each
   **Playbook-context** task node dispatch, resolve the stage agent once with an
   empty pin and stamp the returned machine on every retry's `RunSpec`.
   `cmd/fort` injects the existing live machine registry into both Engine and
   graph execution. Static flows remain local as required by spec 022. With no
   registry, placement remains empty and single-machine behavior is unchanged.
   Route preview stays pure and model-free.
2. Add a first-class Playbooks destination to the authenticated React gateway.
   Load the full catalog through the existing sealed `/api/playbooks` contract,
   show stages, task-type branches, model, memory, trigger, delivery, and plan
   gate, and support the existing immutable save/duplicate operations. No
   plaintext catalog data enters Vercel server components or logs.
3. Reserve “Needs you” for waiting gates and recent failures. Keep older failed
   assignments visible as historical `Failed` outcomes, but do not rank every
   historical failure ahead of newer working or delivered work. Use the same
   48-hour recent-failure window on web, iPhone, and Mac.
4. Fix the macOS `List(selection:)` tag type so Command Deck, Projects, Today,
   Crew, and Playbooks change the detail destination when clicked. Machine
   roster rows remain status, not execution pins; unpinned directions are
   intentionally placed across the mesh.
5. Present the relay daemon as the secure entry point to an all-machine deck:
   the primary toolbar says `All machines`, shows mesh reachability, and keeps
   the daemon name/fingerprint in subordinate connection details.

## Invariants

- Machine placement remains deterministic and makes zero model calls.
- Only graph `task` nodes invoke the runtime.
- Explicit machine pins on direct tasks retain spec-022 semantics.
- Gateway application payloads remain E2E encrypted client-to-daemon.
- Historical failures are never rewritten or hidden; only their attention
  priority and wording change.
- Successful stage placement is appended as a node-scoped event. Placement or
  dispatch failure terminally marks the node and parent run instead of leaving
  an assignment stuck in `running`.
- `core` does not import `ui` or concrete `exec` packages; `ui` keeps its
  existing architecture seam.

## Affected files

- `core/graph/executor.go` and tests; `cmd/fort/main.go` wiring.
- `gateway/web/lib/command-deck.ts`, `components/board-client.tsx`,
  `components/command-deck-surface.tsx`, `app/globals.css`, and gateway tests.
- `ui/page.go` and source-contract tests for the built-in daemon web client.
- `ui/apple/FortKit/Sources/FortKit/CommandDeck.swift` and contract checks.
- `ui/apple/macOS/FortWindow.swift`, `ui/apple/iOS/BoardView.swift`, and Apple
  project/UI tests.

## Test criteria

- A Playbook graph task on an agent offered only by a remote node dispatches
  with that node's machine name; placement happens once across retries;
  placement errors fail without invoking the runtime; nil placement and static
  flows preserve local behavior.
- Gateway Playbooks navigation loads through the sealed client and its primary
  controls work at desktop and phone widths.
- A newer success/working run appears ahead of old failures; only failures from
  the last 48 hours contribute to the attention inbox on every client.
- A macOS UI test clicks Projects and Playbooks and observes their detail
  headings.
- `go test ./...`, focused race tests, gateway tests/typecheck/build, FortKit
  checks, generated Apple builds, and visual interaction QA pass.

## Rollback

Revert this spec's implementation commit. The placer is optional, the gateway
and Apple changes are presentation-only, and no persisted schema or relay
protocol changes are introduced.
