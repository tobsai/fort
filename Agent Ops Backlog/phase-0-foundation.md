# Phase 0 — Reconnaissance & setup

**Goal:** lay the foundation for a native build — stand up the Fort monorepo, learn exactly how to drive each agent CLI headless, and harvest what Multica and Agent Portal/Postern already solved so you borrow instead of reinvent.
**Exit gate:** you can run all four CLIs (`claude`, `codex`, `openclaw`, `hermes`) headless and capture their stdout — and you know precisely how `fort-exec` will spawn, stream, and signal them.
**Why first:** going native means `fort-exec` (AO-014) is the riskiest ticket. This phase de-risks it before you write it.

---

### AO-001 · Set up the Fort monorepo
- **Type:** task · **Pri:** P0 · **Est:** M · **Labels:** dev → Codex · **Depends:** AO-090
- **Do:** One repo with hard module seams: `core/`, `exec/`, `ui/`, `rules/`, `flows/`, `cmd/fort/`, `docs/notes/` (§6.8 of spec). CI, lint, test scaffold.
- **Acceptance:**
  - [ ] Repo builds; module directories exist with placeholder packages.
  - [ ] `core` cannot import `ui`; `core`→`exec` only via the `Runtime` interface (enforced by lint/package boundaries).

### AO-002 · Runtime recon — drive each CLI headless ⭐
- **Type:** spike · **Pri:** P0 · **Est:** M · **Labels:** spike → Hermes · **Depends:** —
- **Do:** For each of `claude`, `codex`, `openclaw`, `hermes`: find the headless/non-interactive invocation, how it streams stdout, how to feed input (for HITL), exit-code semantics, and auth/env needs.
- **Acceptance:**
  - [ ] `docs/notes/runtime-recon.md` documents the exact command, streaming behavior, stdin/signal path, and exit semantics per CLI.
  - [ ] A throwaway script runs one real task on each CLI headless and captures output.

### AO-003 · Study Multica's Apache-2.0 daemon; decide what to vendor ⭐
- **Type:** spike · **Pri:** P0 · **Est:** M · **Labels:** spike → Hermes · **Depends:** —
- **Do:** Read Multica's daemon (Go, Apache-2.0). Identify the reusable hard parts — CLI detection, PTY handling, stdout parsing, lifecycle. Decide: thin study vs. vendor/fork specific packages under license.
- **Acceptance:**
  - [ ] `docs/notes/multica-daemon.md` with a borrow/skip decision per component + license attribution plan.
  - [ ] AO-014 re-estimated against that decision.

### AO-004 · Extract `fort-ui` contracts from Agent Portal/Postern
- **Type:** spike · **Pri:** P1 · **Est:** M · **Labels:** spike → Hermes · **Depends:** —
- **Do:** From `agent-portal` (and Postern, if added to a session): capture the chat API shape (`CHAT_API_SPEC.md`), SSE/WS event model, board task model, OpenClaw channel design, and iOS shell structure — as contracts to copy, not code to port.
- **Acceptance:**
  - [ ] `docs/notes/interface-contracts.md` distills each contract `fort-ui` will reimplement.

### AO-005 · Provider keys + secrets baseline
- **Type:** task · **Pri:** P0 · **Est:** S · **Labels:** errand → OpenClaw · **Depends:** —
- **Do:** Configure provider API keys per CLI; decide where keys live (env / secret store); keep them out of the repo.
- **Acceptance:**
  - [ ] Each CLI can call its provider headless.
  - [ ] No keys committed; `.env.example` documents required vars.

### AO-006 · Runtime topology decision (local vs VPS)
- **Type:** decision · **Pri:** P1 · **Est:** S · **Labels:** decision → you · **Depends:** AO-002
- **Do:** Decide where long-running agents execute — your machine, or a persistent box (for "assign and walk away" beyond your laptop's uptime).
- **Acceptance:**
  - [ ] Decision recorded; if a box, provisioned with the CLIs + keys.
