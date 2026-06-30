# Phase 2 — `fort-core` task-graph + gates

**Goal:** add the deterministic DAG engine on top of the router. Flows sequence work, branch on deterministic checks, pause at human gates, and retry — with inference firing **only** at `task` nodes.
**Exit gate:** one multi-agent flow runs unattended except at gates.
**Anchor:** your game-starter pipeline doctrine generalized — "orchestration is coordination, not generation."

---

### AO-021 · DAG model + executor
- **Type:** task · **Pri:** P0 · **Est:** L · **Labels:** dev → Codex · **Depends:** AO-015, AO-016
- **Do:** Flow = nodes + conditional edges. Node types: `task`, `gate`, `check`, `transform`, `fanout`, `fanin`. Topological execution with persisted `node_run` state; resumable.
- **Acceptance:**
  - [ ] A 3-node linear flow runs end to end; state persisted per node.
  - [ ] Only `task` nodes invoke `Runtime`/inference (asserted).
  - [ ] A flow resumes after a process restart.

### AO-022 · `check` node (deterministic predicates)
- **Type:** task · **Pri:** P0 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-021
- **Do:** Predicates: command exit code, file exists, regex match, "tests pass". Drives conditional edges.
- **Acceptance:**
  - [ ] Pass/fail branch taken from a real command's exit code.
  - [ ] No inference involved.

### AO-023 · `gate` node (human approve/edit/reject)
- **Type:** task · **Pri:** P0 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-021
- **Do:** Pause the flow; expose approve / edit / reject; resume on signal. Until `fort-ui` exists, drive via `fort gate approve|reject <id>` / API.
- **Acceptance:**
  - [ ] Flow halts at a gate and resumes on approval.
  - [ ] Reject routes the alternate edge; edit mutates node input before resume.

### AO-024 · `transform` node
- **Type:** task · **Pri:** P1 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-021
- **Do:** Deterministic data move/format step (generalize the pipeline's normalize/catalog). Records inputs→outputs+hash.
- **Acceptance:**
  - [ ] A transform step records inputs, outputs, and a content hash.

### AO-025 · Retry + escalation semantics
- **Type:** task · **Pri:** P1 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-021
- **Do:** Per-`task` retry (max N), then route to an escalation `gate`. Backoff + idempotency guard.
- **Acceptance:**
  - [ ] A failing task retries N times, then escalates to a human gate.

### AO-026 · Port game-starter pipeline as Flow #1
- **Type:** task · **Pri:** P1 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-022, AO-024
- **Do:** Encode `batch_uniformity → normalize → catalog → import_drop` as a Fort DAG (mostly `transform`/`check`; generation stays outside Fort).
- **Acceptance:**
  - [ ] The asset pipeline runs under Fort, deterministic except any chosen `task` node.

### AO-027 · Author "ship a feature" Flow #2
- **Type:** task · **Pri:** P1 · **Est:** M · **Labels:** design → Claude · **Depends:** AO-022, AO-023
- **Do:** Encode `spec(claude) → plan_gate → implement(codex) → tests(check) → review(claude) → merge_gate → deploy(transform)` with the fail→retry→escalate branch (§6.4).
- **Acceptance:**
  - [ ] Flow runs unattended except at `plan_gate` and `merge_gate`. *(Exit gate.)*

### AO-028 · Scheduler (cron/once triggers)
- **Type:** task · **Pri:** P2 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-021
- **Do:** Trigger flows on a cron or one-shot schedule (for "assign and walk away" / recurring research digests).
- **Acceptance:**
  - [ ] A flow fires on a cron schedule and on a one-shot time.
