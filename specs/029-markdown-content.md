# 029 — Markdown task content (safe-subset rendering)

**Status:** design approved in brainstorm (Toby, 2026-07-08) — pending written-spec review.
**Governed by:** [021-fort-native](021-fort-native.md) · builds on the board ([025-board-ux](025-board-ux.md)) and the run drawer ([027-run-drill-down](027-run-drill-down.md)).

## Goal
Let task/backlog **bodies** carry markdown and render it — safely — wherever
bodies are shown. Today `task.Body` / `backlog_item.body` are stored but **never
displayed anywhere** on the board; this makes the body a first-class, formatted
surface (headings, emphasis, code, lists, links).

## Non-goals (v1 — YAGNI)
- **No full CommonMark.** A deliberate, bounded subset (below). No tables,
  blockquotes, images, HTML passthrough, or nested-list edge cases in v1.
- **No external markdown library / JS.** A small in-page renderer only — no new
  script dependency to keep XSS-safe.
- **No markdown in titles.** Titles stay plain text (one line, already escaped).
- **No server-side rendering.** Bodies are stored raw; rendering is client-side
  in the served page. (The raw body remains the source of truth.)

## Approach

### The renderer (`md(src)` in the served page)
A single function added to the board's `<script>`, **escape-first**:
1. HTML-escape the entire source with the existing `esc()` (`& < > " '`) — so no
   raw markup can ever reach the DOM.
2. Apply the safe subset to the **escaped** text via ordered, bounded regexes:
   - **Fenced code** ```` ``` ```` blocks → `<pre><code>` (extracted first, held
     out of further processing).
   - **Headings** `#`..`######` at line start → `<h3>`..`<h6>` (capped so page
     hierarchy isn't hijacked).
   - **Inline code** `` `x` `` → `<code>`.
   - **Bold** `**x**` / **italic** `*x*` / `_x_`.
   - **Links** `[text](url)` → `<a>` **only** when `url` matches `^https?://` —
     otherwise rendered as literal text. Every link gets
     `rel="noopener nofollow" target="_blank"`.
   - **Lists** — consecutive `- ` / `* ` lines → `<ul><li>`; `1. ` → `<ol><li>`.
   - Remaining newlines → paragraph/`<br>` breaks.
3. Return an HTML string that is safe to assign to `innerHTML` because step 1
   already neutralized all source markup and steps 2's regexes only ever emit a
   fixed, closed set of tags around already-escaped text.

Backtick constraint: the board is a Go raw-string constant (`boardHTML`), so the
implementation must build any literal backtick via `String.fromCharCode(96)` (or
equivalent) rather than a raw `` ` `` in the source.

### Where it renders
- **Backlog / Ready items** — a body preview (first ~3 lines, `md()`-rendered,
  clamped) under the title.
- **Run drawer (027)** — the run's body (if any) rendered at the top of the
  drawer.
- **Dashboard (031)** — Define preview + Ready/In-progress bodies use `md()`.

Titles everywhere stay `esc()`-only plain text.

### Security
The one and only sink is `md()`, which is escape-first. A test corpus of
injection payloads (`<script>`, `<img onerror>`, `javascript:` links,
`](javascript:…)`, attribute-breakouts, backtick tricks) must all render inert.
This is the same class of bug 025 (XSS in card fields) and 027 (unescaped
`node_id`) hit — so the renderer is treated as the security-critical unit and
gets its own adversarial test.

## Architecture (respects the seams)
- **`ui/page.go`** — add `md(src)` to the `<script>`; call it in the item/body
  render paths. Pure frontend; no Go type or endpoint change (bodies already
  flow to the client via existing contracts).
- No `core`/`exec` changes: bodies are already persisted (`task.Body`,
  `backlog_item.body`) and already serialized where needed.

## Decisions
- **D1 — escape-first, closed tag set.** Escape the whole source, then emit only
  a fixed set of tags around escaped text. A mis-parse can only ever *fail to
  format*, never inject.
- **D2 — safe subset, no library.** Headings/emphasis/code/lists/links cover
  real task notes without a dependency to audit.
- **D3 — links are http(s)-only, `noopener nofollow`.** `javascript:`/`data:`/
  relative URLs fall back to literal text.
- **D4 — bodies stored raw.** Source of truth is the raw markdown; rendering is a
  view concern, so nothing to migrate and other clients keep the raw text.
- **D5 — titles are not markdown.** Keeps the one-line, high-frequency field
  trivially safe and layout-stable.

## Affected files
- `ui/page.go` — the `md()` renderer + body render sites (backlog/drawer; the
  dashboard sites land with 031).
- `ui/page_test.go` or a JS-behavior harness note — the injection corpus is
  verified by serving the page and asserting inert output (see test criteria).
- `README.md` / `docs/notes/*` — one line noting markdown bodies.

## Test criteria
- Rendering: `# H` → an `<h3>`; `**b**`/`*i*` → `<strong>`/`<em>`; `` `c` `` and
  fenced blocks → `<code>`/`<pre>`; `- a`/`- b` → a `<ul>` with two `<li>`;
  `[t](https://x)` → an `<a href="https://x" rel="noopener nofollow">`.
- Security (the critical set): `<script>alert(1)</script>`, `<img src=x
  onerror=alert(1)>`, `[x](javascript:alert(1))`, `[x](data:text/html,…)`, a
  title-like `"><svg onload=…>`, and a body mixing all of the above **all**
  render with no executable markup and no non-http link — verified against the
  served page output.
- Regression: an empty/whitespace body renders nothing (no stray tags); a plain
  body with no markdown renders as escaped text unchanged.
- `go build ./...` succeeds (no stray backtick broke the Go raw string);
  `go test ./ui/...` green.

## Rollback
Additive and frontend-only. Revert the `md()` calls (bodies simply stop showing,
or show as `esc()` plain text) — no data, endpoint, or type change to undo.
