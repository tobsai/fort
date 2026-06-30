# Phase 3 — `fort-ui` interface (informed by Agent Portal/Postern)

**Goal:** build Fort's own interface module — chat, board, gate inbox, live feed, iOS — on top of `fort-core`'s event bus, **copying the proven contracts** from Agent Portal/Postern (AO-004) rather than porting their code.
**Exit gate:** you run a full day entirely from `fort-ui`.
**Note:** this is a Fort module, not a separate app. Postern/Agent Portal inform it; they are not dependencies.

---

### AO-031 · `fort-ui` ↔ `fort-core` event/command contract
- **Type:** task · **Pri:** P1 · **Est:** M · **Labels:** design → Claude · **Depends:** AO-016
- **Do:** Define the event schema `fort-ui` subscribes to (sourced from `fort-core`'s append-only `event` log) and the command API it calls back. Informed by Agent Portal's SSE/WS model (AO-004).
- **Acceptance:**
  - [ ] Schema doc published; `fort-core` emits conformant events; a test client replays a run.

### AO-032 · Board module (live runs/nodes/agents)
- **Type:** task · **Pri:** P1 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-031
- **Do:** Reimplement Agent Portal's board model (`todo/in-progress/done` → run/node states) as a live view over `fort-core`.
- **Acceptance:**
  - [ ] Board renders live Fort runs/nodes/agents in real time.

### AO-033 · Live-feed transport (SSE/WS)
- **Type:** task · **Pri:** P1 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-031
- **Do:** Implement the SSE/WebSocket transport (contract from AO-004) carrying the event log to `fort-ui`.
- **Acceptance:**
  - [ ] Board + chat update in real time as a run progresses.

### AO-034 · Chat surface wired to `fort-core`
- **Type:** task · **Pri:** P1 · **Est:** L · **Labels:** dev → Codex · **Depends:** AO-031
- **Do:** Reimplement the chat surface (contract from `CHAT_API_SPEC`). Chat talks to the orchestrator and to single agents; a message can **compile to a flow from templates** (deterministically — not via an LLM planner).
- **Acceptance:**
  - [ ] Chat to a specific agent works (the "Claude for chat/design" surface).
  - [ ] Saying "ship X" instantiates the matching Fort flow.

### AO-035 · Gate-inbox UI (approve/edit/reject → signal)
- **Type:** task · **Pri:** P1 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-023, AO-031
- **Do:** Surface `gate` and `blocked` nodes as an inbox; actions call `fort-core`'s `signal()`.
- **Acceptance:**
  - [ ] Approving/rejecting from `fort-ui` resumes/branches the paused flow.

### AO-036 · OpenClaw channel (informed by Agent Portal)
- **Type:** task · **Pri:** P2 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-004
- **Do:** Reimplement the OpenClaw channel (from `packages/openclaw-channel-agent-portal`) so OpenClaw messages flow into Fort and map to tasks.
- **Acceptance:**
  - [ ] OpenClaw messages create/route Fort tasks.

### AO-037 · iOS shell pointed at Fort
- **Type:** task · **Pri:** P2 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-032
- **Do:** Bring Agent Portal's Swift iOS shell into `fort-ui/ios`, pointed at `fort-core` for mobile visibility + gate approvals.
- **Acceptance:**
  - [ ] iOS app shows live runs and can approve a gate. *(Mobile is a Multica gap — your edge.)*
