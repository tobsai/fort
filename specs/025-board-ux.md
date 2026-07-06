# 025 — Board UX: kanban, agent picker, light/dark theming

**Status:** approved in brainstorm (Toby, 2026-07-06) — pending written-spec review.
**Governed by:** [021-fort-native](021-fort-native.md) · builds on the served board (`ui/`).

## Goal
Make the served board feel like a real, polished control surface. Three changes,
all in the front end:
1. The flat run list becomes a **kanban**: runs grouped into status columns.
2. The compose bar gains an **agent picker** so you can choose which agent runs a
   task (today you can only pick the machine).
3. The board supports **light and dark themes**, holding Fort's command-center
   identity in both.

Guiding principle throughout: **simplicity of controls and feedback**. The kanban
is the feedback (where each run is); the compose bar is the controls (machine,
agent, prompt, send). Nothing louder than it needs to be.

## Non-goals (v1 — YAGNI)
- No drag-and-drop between columns. Runs progress by their own state; the board is
  a live view, not a manual state machine.
- No per-run elapsed timers, filtering, search, or pagination. (Deferred; keeps
  this a pure front-end change with no new board data.)
- No new endpoints and no server behavior changes. Everything renders from data
  the board already serves.
- Not the task-breakdown feature — that is a separate capability (spec 026).

## Approach — all in `ui/page.go`
`ui/page.go` serves the board's HTML + CSS + JS as one string. It already fetches
`/api/board` (runs with `status`), `/api/machines` (per-machine `agents`), posts
to `/api/chat` (`{text, machine, agent}` — `agent` already accepted server-side),
and posts gate decisions to `/api/gate`. All three changes fit here.

### Layout
- **Top bar:** brass `FORT` wordmark + plane pill, live status counts, the machine
  roster (name + health dot + agents), and the theme toggle (top-right).
- **Kanban:** the four status columns fill the main area, replacing today's flat
  RUNS panel.
- **Compose bar:** a single full-width row below the kanban — machine select,
  agent select, prompt input, send. This is the whole control surface.
- The current separate **gate-inbox** panel is **folded into the Blocked column**
  (see D6): one place for "needs you", nothing duplicated. The live event feed, if
  shown today, is unchanged by this spec.

### 1. Kanban
Group `board.runs` by status into four fixed columns, left to right:

| Column | Statuses | Accent |
|---|---|---|
| **Queued** | `queued` | neutral |
| **Running** | `running` | amber (warning) |
| **Blocked** | `blocked` (awaiting a gate) | blue (needs you) |
| **Done** | `succeeded`, `failed`, `canceled` | green / red / neutral left edge per outcome |

- One **Done** column (Toby's choice); outcome is shown by the card's left accent
  colour, not a separate column.
- Each run is a **card**: title, agent name, machine tag, and a 2px left accent in
  the column/outcome colour. No loud status badges — the column and accent carry
  the state.
- **Blocked** cards surface the gate inline with **approve / reject** actions that
  post to `/api/gate` (the data is already in `/api/gates`; wire the card to it).
- Columns are vertical stacks; **Done** lists newest first. Empty columns show a
  quiet placeholder, not nothing.
- Live updates keep working: the board already refreshes from `/api/board` /
  `/api/events`; grouping is recomputed on each refresh.

### 2. Agent picker
A second `<select>` in the compose bar, beside the machine picker:
- Options: **"auto agent"** (default, value `""` — unchanged routing behaviour)
  plus the distinct agents offered across the mesh (union of `agents` from
  `/api/machines`).
- Selecting one **forces** that agent (sends `agent` in the `/api/chat` body,
  matching the CLI's `--agent`). "auto agent" lets the deterministic rules route.
- When a machine is selected, the agent list **filters to that machine's agents**
  so an impossible combo (agent a machine doesn't offer) can't be chosen; "any
  machine" shows the full union. If the current agent choice becomes invalid after
  a machine change, it resets to "auto agent".

### 3. Light / dark theming
Replace the hardcoded dark palette with CSS custom properties and two themes:
- Define all colours as `--fg`, `--bg`, `--panel`, `--card`, `--line`, `--muted`,
  `--brass` (accent), and status colours `--run`, `--block`, `--ok`, `--fail`,
  under `:root` (dark) and `:root[data-theme="light"]` (light). Every rule reads a
  variable — no literal colours in component CSS.
- **Default follows the OS**: `prefers-color-scheme` sets the initial theme.
- A single small **toggle** (sun/moon icon, top-right) flips `data-theme` and
  **persists to `localStorage`**; the persisted choice wins over the OS default on
  next load. This is the only control the theming adds.
- Both palettes keep the command-center identity: a brass accent, semantic status
  colours (amber/blue/green/red) tuned for contrast in each mode.

### Polish (an explicit acceptance criterion, not a nice-to-have)
Verified visually against the running board, in **both** themes:
- one consistent palette per theme (all via the CSS variables above);
- an 8px spacing grid; a tight type scale (~11–13px meta, ~12.5–13px card titles);
- status by column + left accent, not heavy badges;
- real hover and focus-visible states on cards, selects, input, and buttons;
- subtle transitions (colour/opacity), no gradients or shadows-as-decoration;
- WCAG-AA text contrast in both themes; controls fully keyboard-operable.

## Decisions
- **D1 — kanban is a view, not manual.** Columns reflect run state; no drag. Fort
  runs progress deterministically, so a draggable board would misrepresent truth.
- **D2 — one Done column.** Outcome via left-accent colour (Toby's choice), keeping
  four tidy columns and honouring "simple feedback".
- **D3 — agent picker forces the agent.** Mirrors `--agent`; "auto agent" is the
  default and preserves today's rules-based routing. Machine-aware filtering
  prevents invalid pins.
- **D4 — theme = OS default + persisted toggle.** No account setting, no server
  round-trip; `localStorage` only. One icon control.
- **D5 — front-end only.** No new/changed endpoints; the board renders from
  existing `/api/board`, `/api/machines`, `/api/gates`. Keeps `ui` a thin view over
  the ports and the change low-risk.
- **D6 — Blocked subsumes the gate inbox.** A run awaiting a gate is exactly a
  blocked run, so its approve/reject lives on its card in the Blocked column; the
  separate gate-inbox panel is removed. One surface for "needs you" — simpler
  feedback, no duplicated list.

## Affected files
- `ui/page.go` — the entire change: themed CSS variables + toggle, kanban
  grouping/rendering, agent `<select>` + machine-aware filtering, gate actions on
  Blocked cards.
- `ui/ui_test.go` — confirm/extend server-side contract tests that back the board:
  `/api/chat` honours a supplied `agent`; `/api/board` exposes each run's `status`;
  `/api/machines` exposes per-machine `agents`. (The board JS itself is verified
  live, not unit-tested.)

## Test criteria
- `go test ./ui/...` green, including: `/api/chat` with `{agent:"codex"}` dispatches
  (or boards) a codex run; `/api/board` payload carries `status`; `/api/machines`
  carries `agents`.
- Live board check (both themes): runs land in the correct column by status; a
  gate-blocked run appears in **Blocked** and approve/reject from the card resolves
  it; the agent picker forces the chosen agent and filters by selected machine;
  "auto agent" preserves rules routing.
- Theme: initial theme follows `prefers-color-scheme`; the toggle flips and
  persists across reload; both themes pass an AA contrast check on text.
- `go test ./...` and `go vet ./...` stay green (no server regressions).
- Full existing board behaviour (feed, counts, machine roster, gate inbox) intact.

## Rollback
Additive, front-end only, confined to `ui/page.go`. Revert the commit to restore
the current board; no data or API migration either way.
