# Fort Design System

The visual language for all Fort surfaces: web control plane, iPhone app, Mac app. Everything here is normative — implement from these tokens, not from screenshots.

## Principles
1. **The UI answers "what needs me?"** — attention hierarchy beats information density. Blocked-on-human gets the biggest pixels; running-and-fine is quiet; idle is dimmest.
2. **Human words, same API.** assignment (run), sign-off / "Needs you" (gate), Up next (backlog), Start (dispatch), Draft a plan (breakdown), checkpoint (DAG node). Engine/model names appear at assignment time, in mono, then recede.
3. **Progress = checkpoints you accepted.** Never render an agent-estimated percentage.
4. **One status grammar everywhere.** Learn it once: color + motion carry state on sigils, dots, pills, blocks, and borders.

## Color tokens
Surfaces
- `--bg-canvas: #07090e` (page behind cards) · `--bg: #0b0e14` (app background) · `--panel: #12161f` (cards)
- Lines: `--line: #1a212e` (default) · `--line-2: #212938` · `--line-raised: #26314a` (active/bordered cards) · `--outline: #303848` (buttons, chips)
Text
- `--text: #e8ebf2` · `--text-body: #b8bfce` · `--text-muted: #8b93a5` · `--text-faint: #687183` · `--text-disabled: #4a5262`
Brand
- `--brass: #c9a35c` (primary actions, selection, wordmark) · `--brass-bright: #dcb877` (hover, links, accents) · tint `rgba(201,163,92,.12–.16)`
Status (semantic — never swap)
- **Running / ready / memory**: `--blue: #6fa8ff`, tint `rgba(111,168,255,.07–.14)`, queued block `#2a3650`
- **Needs you / warning / error-attention**: `--amber: #e0a458`, tint `rgba(224,164,88,.13–.14)`. Amber never animates.
- **Accepted / good / online**: `--green: #57b98a`
- **Failed / now-line / slipping**: `--red: #d96a6a`
- **Idle / not started**: `#56617a` (marks, dashed borders) · `#303848` (rings, dots)
On-color text: on brass/amber `#0b0e14`/`#12100a` · on blue `#07101f` · on green `#07120c`.

## Typography
- **UI**: 'Instrument Sans' (Google Fonts), weights 400–700.
- **Data**: 'Spline Sans Mono' — machine names, timestamps, ids, metric numerals, model names, stage numbers. Mono is a signal: "this is data, not prose."
- Scale: 11.5 / 12 / 12.5 / 13.5 / 14 / 15 / 16 / 17 / 18 / 22px. Body 13.5px/1.5–1.6; titles 15.5–17px/600; metric numerals mono 22px/600.
- Section labels: 11–12px / 600 / uppercase / letter-spacing .08–.1em, colored by section semantics (amber for "Needs you", muted otherwise).
- Wordmark: FORT, mono 700, letter-spacing .22em, brass-bright.
- Minimums: web/Mac 11.5px (mono labels) and 12px (sans); mobile body ≥13px, hit targets ≥44px.

## Shape & space
- Radius: 4 (bar segments) · 6 (model chips) · 7–9 (buttons, schedule blocks) · 10 (cards) · 12 (large cards, mobile cards) · 14 (Mac window) · 16–22 (pills, chips) · 50% (sigil rings, dots).
- Spacing on an 8-ish grid: 8/10/12/14/16/20/22px. Card padding 14–20px. Layout gaps via flex/grid `gap`, never margins between siblings.
- No shadows for hierarchy (windows/phones excepted) — use background steps + borders.

## Status grammar (the one to memorize)
| State | Color | Motion |
|---|---|---|
| Working / running | blue | white arc racing around the ring (sigils); white sheen sweeping the block (schedule); dot opacity pulse |
| Needs you | amber | none — amber is loud enough |
| Accepted / delivered / online | green | none |
| Queued / up next | #2a3650 | none |
| Scheduled (recurring) | dashed #56617a border | none |
| Idle / not started | grey | none |

Animations:
- `@keyframes spinrace { to { transform: rotate(360deg) } }` — ring overlay: `conic-gradient(transparent 0 70%, #fff 82%, transparent 94%)` masked to a 3–4px ring (`radial-gradient` mask), 2.2s linear infinite.
- Sheen: `linear-gradient(105deg, #6fa8ff 42%, #e4efff 50%, #6fa8ff 58%)` at `background-size: 220% 100%`, animate background-position, 2.6s linear.
- `@keyframes dotpulse { 50% { opacity: .35 } }`, 1.6s.

## Sigils (project identity marks)
Deterministic mark from the project name; **fill = the project's current status color**; the surrounding ring (2px, 50% radius, 4–8px padding) matches. Generation: FNV-1a hash → xorshift32 → 5×5 grid, 3 columns mirrored, cell on if rand > 0.55; SVG rects: cell u = size/5, inset 4%, 88% square, rx 20%. Sizes: 26–30px (rows) / 34px (legend) / 42px (cards).

## Components
- **Buttons**: primary brass (600, radius 8–12); confirm green; secondary 1px `--outline`; tertiary text-only muted; outlined-brass for additive actions ("Assign work", "＋ New playbook"). Mobile: full-width, 15px padding.
- **Pills** (status): 12px/600, radius 20, tinted bg + status color text.
- **Needs-you card**: panel bg, raised border, 3px amber left border, title 15.5–16px/600, mono relative time right, body 13.5px with project name in brass-bright 600, action row gap 8.
- **Checkpoint bar**: equal flex segments 8px, radius 4 — green accepted / amber awaiting sign-off / blue in progress / #212938 not started. Caption 12.5px muted. Dots variant: 9px circles, filled or outlined.
- **Agent + model chips**: agent = sans 600 in outlined pill (radius 14–16); model = mono 11–11.5px chip on `--line` bg (radius 6). Chain stages joined by `→` in #56617a.
- **Schedule blocks**: 36px, radius 7, per status grammar; "open capacity" = 1px `--line` border, italic faint text.
- **Toggles**: 34×20 (web) / 44×26 (mobile), brass track when on, `--bg` knob.
- **Sidebar (Mac)**: 210px, item 13.5px, selected = brass tint bg + brass-bright text; count badge amber.
- **Tab bar (iPhone)**: 5 items, center brass circular "Direct" action (34px, raised); active item brass-bright.

## Form-factor rules
- **Web**: full editor surface — playbook editing, scorecards, week/today timelines.
- **Mac app**: same density as web inside dark native chrome (dark titlebar #12161f, traffic lights); sidebar nav: Deck / Projects / Today / Crew / Playbooks.
- **iPhone**: triage only — approve, redirect, brief, switch route. No diff review, no playbook editing. Content starts ≥64px below the notch; primary action fixed at bottom.
