# Handoff: Fort Dashboard Redesign

## Overview
A redesign of Fort's web control plane (`ui/page.go`) around a delegation model: the landing surface answers "what needs me?", projects carry generated identity sigils with a status-ring grammar, progress is measured in human-accepted checkpoints (never agent-estimated %), agents get performance scorecards with versioned methods, and schedule views (Today / Week) show upcoming work per agent — including predicted moments the human will be needed.

## About the Design Files
`Fort Redesign.dc.html` (+ `support.js`, its render runtime) is a **design reference created in HTML** — a canvas of mockups showing intended look and behavior, not production code. The task is to **recreate these designs inside Fort's existing environment**: the Go-served single-page board in `ui/page.go` (vanilla HTML/CSS/JS over the existing HTTP/SSE API), keeping the module seams (`ui` imports no execution components). Open the .dc.html in a browser to view all mockups; they are grouped into "turns" (newest at top): Turn 3 = Today schedule, Turn 2 = performance + week, Turn 1 = three landing-page directions plus the design-position/vocabulary/sigil legend.

## Fidelity
**High-fidelity.** Colors, type sizes, spacing, and copy tone are intentional and should be matched. Data shown (projects, agents, metrics) is illustrative.

## Core design decisions (apply everywhere)
1. **Vocabulary — same API, human words.** run/task → *assignment*; gate → *sign-off / "Needs you"*; backlog → *Up next*; dispatch → *Start*; breakdown → *Draft a plan*; flow/DAG node → *project plan / checkpoint*; agent CLI/engine → shown at assignment time only.
2. **Attention hierarchy.** Blocked-on-human items get the largest, most prominent treatment; running-and-fine gets a quiet pulse; idle is dimmest.
3. **Progress = accepted checkpoints.** Never render an agent-estimated percentage. Progress bars are segmented: accepted (green) / awaiting sign-off (amber) / in progress (blue) / not started (hollow/dark).
4. **Sigil grammar — identity ≠ status.** Each project gets a deterministic generated mark (identicon); a surrounding **ring** carries state: blue with a white streak racing around the ring = working, amber = needs you (warning), green = all accepted, dark gray #303848 = idle.
5. **Type scale.** Base UI 13–15px sans; titles 16–18px; monospace reserved for data only (machine names, times, ids, metric numerals).

## Screens / Views

### 1a — Command Deck (landing, inbox-first)
- Top bar (padding 14px 22px, 1px #1a212e bottom border): brass FORT wordmark (mono, 15px/700, letter-spacing .22em, #dcb877) · "2 need you" pill (12px, bg rgba(224,164,88,.14), color #e0a458, radius 20px) · spacer · machine dots (7px green #57b98a circle + mono 12px name) · primary button "Give direction" (bg #c9a35c, text #0b0e14, 600 13px, radius 8px, padding 7px 16px).
- Two-pane body: left flex 1.5 "NEEDS YOU" (label 12px/600 uppercase #e0a458, letter-spacing .09em), right flex 1 "PROJECTS" + "CREW", separated by 1px #1a212e.
- **Needs-you card**: bg #12161f, border 1px #26314a, border-left 3px #e0a458, radius 10px, padding 16px 18px. Title 16px/600; relative time right-aligned mono 11.5px #687183; body 13.5px/1.55 #b8bfce with project name in #dcb877 600. Actions row (gap 8px): Accept/Approve (bg #57b98a, text #07120c), secondary outlined (#303848 border), tertiary text-only #8b93a5. Empty state: "That's everything — N agents are working and don't need you." 12.5px #687183.
- **Project row**: sigil (30px) in 4px padding wrapper with 2px status-ring border radius 8px; name 14px/600; one summary line 12px #8b93a5 ("3 of 5 checkpoints accepted · 1 awaiting sign-off").
- **Crew list**: 8px status dot (blue pulsing = working, amber = waiting on you, #303848 = idle), bold name, plain-English activity + elapsed time in #8b93a5.

### 1b — Project Rooms (landing, project-centric)
2×2 grid (gap 16px, padding 22px) of project cards: bg #12161f, radius 12px, padding 20px. Card with pending sign-off uses border #26314a; idle/unassigned uses dashed #303848. Header: sigil 42px + ring · name 17px/600 · agents/machine 12.5px #8b93a5 · status pill right (needs you amber / working blue / delivered green tints at ~13% alpha). Checkpoint bar: equal flex segments 8px tall radius 4px, colors per decision 3; caption 12.5px. One plain-English activity sentence 13.5px #b8bfce. One CTA max per card: filled #e0a458 for review, outlined for watch/assign.

### 1c — The Roster (landing, delegation-first)
Left pane: "Give direction" 18px/600; markdown brief textarea (bg #12161f, border #26314a, radius 10px, 14.5px, min-height 110px); "ASSIGN TO" chip row — "Fort decides" selected by default (1.5px #c9a35c border, rgba(201,163,92,.12) bg) then agent chips (outlined #303848); toggle "Propose a plan first — I'll sign off before work starts" (default ON — this is the plan gate); submit "Hand it off" (bg #c9a35c). Right pane: roster rows per agent — card bg #12161f radius 10px, border-left 3px status color, name 15px/600 + status pill + machine mono right; assignment sentence 13.5px #b8bfce; checkpoint dots 9px (filled green accepted, blue pulsing current, outlined future). Idle agent row is dimmed with an "Assign work" outlined brass button.

### 2a — Crew performance (scorecards)
Header: title + "last 30 days · N assignments" + task-type filter select. 2×2 grid of agent cards: name 16px/600 · `method vN` chip (mono 11.5px, bg #1a212e) · trend right ("▲ improving" #57b98a / "→ steady" #8b93a5 / "▼ slipping" #d96a6a). Metric row (gap 22px): big numerals mono 22px/600 — first-pass accepted %, redirects/assignment, $ per accepted checkpoint — each with an 11.5px #687183 label; 90×34 sparkline (2px polyline, colored by trend). Task-type chips: "best at: X" green tint, "weak: Y" #1a212e/#687183. Footer line 12.5px #8b93a5 explains the number and offers the action: promote a method version, or "Try a method variant…" (outlined brass) on slipping agents. Show sample counts next to percentages in the real build — 30-day samples are small.

### 2b — The Week (schedule, one row per agent)
CSS grid `130px repeat(7, 1fr)`, gap 8px 6px; mono day headers (today in brass). Blocks are 36px tall radius 7px spanning day columns: **active now** solid #6fa8ff (dark text #07101f), **up next** (Ready queue) solid #2a3650 text #b8bfce, **scheduled/recurring** 1.5px dashed #56617a text #8b93a5, **waiting on you** solid #e0a458. Idle capacity renders as an outlined italic "open capacity — assign work" block. Legend in header. Recurring blocks come from Fort's scheduler (spec 008); up-next blocks are the ordered backlog; drag between rows reassigns (POST the item with a new agent).

### 3a — Today (day schedule)
Same grid pattern with 12 hour columns (8am–8pm), NOW as a 2px #d96a6a vertical line with mono "NOW" label. **Top row is "You" (brass label)** and is derived, not planned: solid amber block = a sign-off already waiting; dashed amber = a checkpoint an agent is on pace to reach (ETA from current run). Agent rows: active block (blue, racing white sheen) with inline ETA copy ("→ checkpoint 2 ~2pm"), queued next (blue-gray), waiting-on-you (amber), recurring (dashed), idle (outlined italic). Header summarizes the human's day: "2 sign-offs expected before 3pm · evening is clear."

## Sigil generation (implement in JS)
FNV-1a hash of the project name → xorshift32 PRNG → 5×5 grid, 3 columns generated and mirrored (cell on if rand > 0.55). Render as SVG rects: cell = size/5, inset 4%, width/height 88% of cell, rx 20% of cell, fill = the project's current status color (blue #6fa8ff working, amber #e0a458 needs-you, green #57b98a accepted, grey #56617a idle) — the mark and its ring always match. Deterministic — same name always yields the same mark. Ring is a wrapper border, not part of the SVG.

```js
function sigil(name, size) {
  let h = 2166136261;
  for (const ch of name) { h ^= ch.charCodeAt(0); h = Math.imul(h, 16777619) >>> 0; }
  const rand = () => { h ^= h << 13; h ^= h >>> 17; h ^= h << 5; h >>>= 0; return h / 4294967296; };
  const cells = [];
  for (let x = 0; x < 3; x++) for (let y = 0; y < 5; y++)
    if (rand() > 0.55) { cells.push([x, y]); if (x < 2) cells.push([4 - x, y]); }
  // render cells as <rect> at x*u, y*u (u = size/5), 0.88u square, rx 0.2u, fill #c9a35c
}
```

## Interactions & Behavior
- Approve / Accept checkpoint → existing `POST /api/gate`; Request changes / Redirect opens a text field then posts the decision with a note.
- Start (Up next) → `POST /api/backlog/{id}/dispatch`; Hand it off → `POST /api/chat` (title = first line, body = rest) or `POST /api/backlog`; Draft a plan → `POST /api/breakdown`.
- "Propose a plan first" ON wraps the assignment in a flow with a plan gate before execution.
- Clicking any assignment/run opens the existing drill-down drawer (spec 027) unchanged.
- Live updates ride the existing 3s `/api/board` poll + `/api/events` SSE.
- Running indicators: the sigil ring is blue with a small white arc racing around it — a circular overlay with a conic-gradient arc masked to a ring, rotating 2.2s linear infinite. Active schedule blocks carry a moving white sheen (animated background-position, 2.6s). Status dots pulse opacity 1→.35, 1.6s. Amber is reserved for needs-you and errors; it never animates.
- Hover: cards lift border to #26314a; all buttons have cursor:pointer and a visible focus ring.
- Control-only mode (`fort control`): Run/Draft-a-plan surface the 409 as today.

## State Management
- Board state: reuse existing summary/board/backlog polling; re-bucket into Needs you / Projects / Crew (1a) or project cards (1b).
- New (small) persistence for full vision: project entity (name, checkpoint list, accepted flags), checkpoint accept events, per-agent method version tag on runs, metric rollups (first-pass acceptance, redirects, cost per accepted checkpoint — cost from spec 007 tracking). Scorecards and the "You" row derive from these.
- Theme: existing light/dark CSS-variable system; these mocks show dark. Map colors to variables.

## Design Tokens
Canonical reference: `DESIGN-SYSTEM.md` (tokens, status grammar, sigils, components, form-factor rules). Summary:
- Background #07090e (canvas) / #0b0e14 (page) · panel/card #12161f · lines #1a212e, #212938 · raised border #26314a · outline #303848
- Text: primary #e8ebf2 · body #b8bfce · muted #8b93a5 · faint #687183 · disabled #4a5262
- Brass (brand/identity/CTA): #c9a35c, hover/bright #dcb877
- Status: working #6fa8ff · needs-you #e0a458 · accepted/good #57b98a · failed/now-line #d96a6a · queued block #2a3650
- On-color text: on brass/amber #0b0e14/#07101f · on blue #12100a · on green #07120c
- Type: 'Instrument Sans' (UI) + 'Spline Sans Mono' (data), Google Fonts. Scale: 11.5/12/12.5/13.5/14/15/16/17/18/22px. Uppercase labels: 11–12px/600, letter-spacing .08–.1em.
- Radius: 4 (bar segments) / 7–8 (buttons, blocks) / 10 (cards) / 12 (large cards) / 20 (pills). Spacing: 8px-ish grid (8/10/12/14/16/20/22px).
- Shadows: essentially none — hierarchy via borders and background steps.

## Assets
None required. Sigils are generated at runtime (code above). The brass FORT wordmark is styled text; the existing logo asset (`assets/fort-logo.png`) is unused in these mocks.

## Playbooks (agent + model routing)
See `SPEC-playbooks.md` — playbook pipelines (per-stage agent + model, shared memory, task-type branching), triggers, shortcuts (Quick answer), and the route preview on handoff. Mocks: web turn 4 (4a editor, 4b route preview); mobile/Mac in `Fort Mobile and Mac.dc.html` turn 2.

## Files
- `Fort Redesign.dc.html` — all mockups (open in a browser; canvas with turns 3/2/1 top-to-bottom)
- `Fort Mobile and Mac.dc.html` — iPhone + Mac app form factors (turn 2 playbooks, turn 1 command deck / give direction)
- `SPEC-playbooks.md` — playbooks/routing spec
- `DESIGN-SYSTEM.md` — canonical design system (tokens, grammar, components)
- `support.js`, `ios-frame.jsx` — render runtime for the mockups only; not part of the design
