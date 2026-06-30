# Phase 1 — `fort-core` + `fort-exec` (deterministic routing + native execution)

**Goal:** the deterministic router plus the **native** executor that spawns the agent CLIs itself — no Multica. A new task auto-routes to the right agent and runs locally, streaming to completion.
**Exit gate:** a new task auto-routes (≥90% accuracy) and runs **natively** to completion with live streaming.
**Principle:** the routing path reads only deterministic signals — no model decides routing. Inference happens only when `fort-exec` launches a CLI.

---

### AO-011 · Scaffold `fort-core` service
- **Type:** task · **Pri:** P0 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-001
- **Do:** `fort-core` process: config loader, structured logging, `/health`, graceful shutdown, local HTTP/WS API stub (for `fort-ui` later).
- **Acceptance:**
  - [ ] `fort` boots, loads config, serves `/health`.
  - [ ] CI runs lint + tests on push.

### AO-012 · Routing-rule YAML schema + parser
- **Type:** task · **Pri:** P0 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-011
- **Do:** Implement the §6.3 schema (`version`, `defaults`, ordered `rules`). Parse + validate with line-level errors.
- **Acceptance:**
  - [ ] Loads a ruleset; rejects invalid ones clearly.
  - [ ] Unit tests for first-match-wins + default.

### AO-013 · Deterministic matcher engine
- **Type:** task · **Pri:** P0 · **Est:** L · **Labels:** dev → Codex · **Depends:** AO-012
- **Do:** Matchers: `label`, `path` (glob), `repo`, explicit `@agent`, `size`, `time`, composed with `any`/`all`. First match wins; record the matched rule.
- **Acceptance:**
  - [ ] Table-driven tests cover each matcher + composition.
  - [ ] Returns exactly one `route` + a `route_decision`.
  - [ ] Zero model calls in this path (asserted in test).

### AO-014 · `Runtime` interface + `NativeRuntime` (PTY executor) ⭐
- **Type:** task · **Pri:** P0 · **Est:** XL · **Labels:** dev → Codex · **Depends:** AO-002, AO-003, AO-011
- **Do:** Define the §6.2 `Runtime` interface (`dispatch`/`stream`/`status`/`signal`). Implement **`NativeRuntime`**: spawn `claude`/`codex`/`openclaw`/`hermes` in a PTY with workdir/env; normalize stdout → `RunEvent`s; write stdin for `signal`; track exit code. Reuse Multica's daemon internals per AO-003.
- **Acceptance:**
  - [ ] Each of the four providers runs a real task via `NativeRuntime` and streams normalized events.
  - [ ] `signal` injects input into a running interactive task; cancel works.
  - [ ] Interface is mockable — a `FakeRuntime` powers fast unit tests.
- **Note:** the keystone of going native. De-risked by AO-002 + AO-003.

### AO-015 · Inbox watcher → router → native dispatch
- **Type:** task · **Pri:** P0 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-013, AO-014
- **Do:** Source new tasks (`fort task add`, watched file/dir, or label feed); run matcher → `NativeRuntime.dispatch`. End-to-end, hands-off.
- **Acceptance:**
  - [ ] A new task auto-routes and runs natively with **zero manual assignment**.
  - [ ] Routing accuracy measured on a sample set (target ≥90% — the exit gate).

### AO-016 · State store + route_decision + event log
- **Type:** task · **Pri:** P1 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-011
- **Do:** SQLite with the §6.6 entities (`run`, `node_run`, `route_decision`, `event` append-only). Persist every dispatch + matched rule.
- **Acceptance:**
  - [ ] Every dispatch persisted with matched rule + provider; queryable by run.
  - [ ] `event` is append-only (the future `fort-ui` feed source).

### AO-017 · Author v1 ruleset (the four lanes)
- **Type:** task · **Pri:** P0 · **Est:** S · **Labels:** design → Claude · **Depends:** AO-013
- **Do:** Write the real ruleset: design/chat/research/decision → Claude; feature/bug/refactor/build/ci → Codex; errand/message/home/notify → OpenClaw; memory/longrun/knowledge → Hermes; plus a default.
- **Acceptance:**
  - [ ] Sample tasks for each lane route to the intended agent.
  - [ ] Ruleset committed with comments.

### AO-018 · `fort` CLI (dry-run, runs, logs)
- **Type:** task · **Pri:** P1 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-013, AO-014
- **Do:** `fort task add`, `fort route --dry-run <task>` (prints rule + agent, no dispatch), `fort runs list`, `fort run logs <id>` (tails the stream).
- **Acceptance:**
  - [ ] `route --dry-run` prints the matched rule + target agent.
  - [ ] `run logs` tails a live native run.
