# SPEC — Playbooks: agent + model routing

## Concept
A **playbook** is a reusable pipeline that defines, per stage of the problem-solving toolchain, **which agent** runs it and **which model** it uses, plus what carries forward in shared memory. **Triggers** route incoming direction to a playbook automatically; **shortcuts** are triggers that skip the chain entirely (e.g. a plain question never spins up breakdown → design → build). The human confirms the route on every handoff — one click/tap to change it. Stage outputs keep the existing checkpoint contract: sign-offs still land in "Needs you".

Example (the canonical one): direction arrives → **Hermes** breaks it down using **Codex 5.6 Sol** and stores the plan in shared memory → **OpenClaw** designs using **Fable** → **Claude Code (Sonnet)** or **Codex (5.6 Sol)** builds, chosen by task type.

## Entities
- **Playbook**: `{ id, name, isDefault, planGate: bool, stages: Stage[], trigger: Trigger }`
- **Stage**: `{ order, name (Break down | Design | Build | custom), assignments: [{ taskType?: string, agent, model }], memory: bool (output persists to shared run memory), skippable: bool + skip condition (e.g. "no UI work in plan") }`
  - A stage with multiple `assignments` branches **by task type** (features → Claude Code · Sonnet; bug fixes → Codex · 5.6 Sol). Default branch required.
- **Trigger**: classifier over incoming direction: `{ kind: question | bug report | feature | research | manual }`. Classification runs on the default breakdown agent/model; the route preview shows the result before anything starts.
- **Shortcut**: a trigger→playbook binding with `checkpoints: none` and `deliver: chat reply` (Quick answer) or a reduced chain (Bug fix skips Design). Each has an enable toggle.
- **Model catalog**: per agent, the list of selectable models (e.g. Hermes: Codex 5.6 Sol…; OpenClaw: Fable…; Claude Code: Sonnet/Opus). Models render in mono type, agents in sans bold — everywhere.

## Behavior
- Handoff flow: user writes direction → Fort classifies → **route preview** renders the chosen playbook as an agent·model chip chain + plan-gate note → "Hand it off". "Change…" opens a playbook picker.
- Quick answer: replies inline in chat; creates no assignment, no checkpoints, nothing on the schedule. Log it in history only.
- Shared memory: stage 1 output (the checkpoint plan) is readable by all later stages of that run; badge `memory ●` (blue) marks stages that persist.
- Editing a playbook never alters in-flight runs; changes apply to the next handoff.
- Scorecards (see main README §2a) attribute metrics to agent+model+method version, so playbook changes are A/B-able.

## UI per form factor
**Web — Playbooks page** (`Fort Redesign.dc.html` turn 4, mock 4a):
- Left rail 250px: playbook cards (selected = brass left border 3px + `default` chip); name 14px/600, meta line 12px muted ("3 stages · plan gate on").
- Editor: title 17px/600 + trigger sentence with `edit` link. Pipeline = horizontal stage cards (bg #12161f, border #26314a, radius 10, padding 14px 16px) joined by `→` glyphs (#56617a), trailing `＋` to add a stage.
- Stage card: mono stage number + name 14px/600; agent pill (13.5px/600, 1px #303848 border, radius 16, `▾`) + model chip (mono 11.5px, bg #1a212e, radius 6, `▾`); one-line description 12px muted. Badges top-right: `memory ●` (blue tint) / `by task type` (neutral). Branching stage lists one row per task type with a 62px muted type label.
- Shortcuts section: uppercase label, rows (bg #12161f, radius 10) = emoji glyph + "When … → Playbook" 13.5px/600 + agent/model summary 12px + brass toggle.

**Route preview on handoff** (mock 4b, and mobile 2a): blue-tinted card (bg rgba(111,168,255,.07), border #26314a) with `ROUTE` label (blue, uppercase), playbook name, `Change…` link (brass on mobile), the chain as chips (web) or numbered rows (mobile), plan-gate note 12px. Quick-answer handoffs show only the ⚡ line ("Quick answer · Hermes · Codex 5.6 Sol — answering now, no checkpoints") and the answer streams into a plain card below.

**Mobile** (`Fort Mobile and Mac.dc.html` 2a): Give-direction screen gains a segmented control `Assignment | ⚡ Quick question` (bg #12161f, selected segment #26314a). Route preview as above; full playbook *editing* is desktop-only — mobile can only switch the route.

**Mac app** (2b): sidebar gains `⛓ Playbooks`. Pane = stacked playbook cards, each with its chip chain on one line; branch noted inline ("/ Codex on bug-type tasks"); shortcuts list with toggles below. Editing interactions mirror web.

## Visual grammar (unchanged tokens)
Blue #6fa8ff = routing/running/memory; amber #e0a458 = needs-you only; brass #c9a35c = primary action/selection; grey chips #1a212e on #12161f cards; agents = 'Instrument Sans' 600, models = 'Spline Sans Mono' 11–11.5px.

## API sketch
- `GET/PUT /api/playbooks`, `POST /api/playbooks/{id}/duplicate`
- `POST /api/route` — body: direction text; returns `{playbookId, stages[], confidence}` for the preview
- `POST /api/chat` gains `playbookId` override; `answer`-kind responses stream back in chat
- Run records gain `{playbookId, stage, agent, model}` per step for scorecard attribution
