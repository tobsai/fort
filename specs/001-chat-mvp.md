# Chat MVP — Talk to Lewis via Fort Dashboard

**ID:** SPEC-001
**Status:** draft
**Author:** Lewis
**Created:** 2026-03-23

## Goal

Get the end-to-end chat loop working: user types a message in the Fort dashboard → message hits Fort core via WebSocket → Orchestrator routes to Lewis agent → LLM responds → response streams back to the dashboard. This is the "prove it works" milestone.

## Current State

- Dashboard scaffold exists (React + Vite, builds to `packages/dashboard/dist`)
- IPC/WebSocket server exists (`packages/core/src/server/index.ts`, port 4077)
- Orchestrator routes chat → agent, LLM client wraps Anthropic API
- Lewis identity files exist locally (`agents/lewis/identity.yaml` + `SOUL.md`)
- 344/344 tests passing

**What's broken:**
1. `~/.fort` directory doesn't exist — no config, no data dir, no API key
2. Core package doesn't emit JS to `dist/` — `composite: true` + `declaration: true` conflict causes silent no-op on `tsc` (build works with `--declaration false` but that breaks composite)
3. CLI `fort portal` command fails (`npx fort` not linked)
4. No API key configured for LLM (needs `~/.fort/.env` with `ANTHROPIC_API_KEY` or config)

## Approach

### Phase 1: Fix the build pipeline
- Fix `tsconfig.json` so `npm run build` in `packages/core` emits JS + declarations to `dist/`
- Verify `fort portal` command starts the server and serves the dashboard
- Ensure `npm run build` at monorepo root builds all packages in order

### Phase 2: Bootstrap local environment
- Create `~/.fort/` directory structure (data, config)
- Configure LLM: use existing `ANTHROPIC_API_KEY` from environment (already set for OpenClaw)
- Create `~/.fort/config.yaml` with minimal settings (dataDir, specsDir, agentsDir pointing to repo)

### Phase 3: Wire the chat loop
- Verify: `fort portal` starts → dashboard loads at localhost:4077
- Verify: WebSocket connects dashboard ↔ core
- Verify: typing a message creates a Task in TaskGraph
- Verify: Orchestrator routes to Lewis agent
- Verify: LLM completes and result streams back to dashboard
- Verify: chat history persists across page reloads (SQLite via MemoryManager)

### Phase 4: Lewis identity
- Ensure AgentFactory loads `agents/lewis/` on startup
- Lewis responds with his SOUL.md personality (not generic Fort default)
- Dashboard shows "Lewis" as the agent name with correct identity

## Affected Files

**Build fixes:**
- `packages/core/tsconfig.json` — fix composite/declaration emit
- `tsconfig.base.json` — may need adjustment
- `package.json` (root) — verify build script ordering

**Environment:**
- `~/.fort/.env` — API key
- `~/.fort/config.yaml` — minimal config
- `agents/lewis/identity.yaml` — already exists (local only, gitignored)
- `agents/lewis/SOUL.md` — already exists (local only, gitignored)

**Possible code changes:**
- `packages/core/src/server/index.ts` — if dashboard serving or WebSocket has bugs
- `packages/core/src/fort.ts` — if startup sequence needs adjustment
- `packages/cli/src/commands/portal.ts` — if CLI entry point is broken

## Test Criteria

1. `npm run build` succeeds across all packages (exit 0)
2. `fort portal --no-open` starts server on port 4077
3. `curl http://localhost:4077` returns dashboard HTML
4. Opening dashboard in browser shows chat UI
5. Sending "Hello" in chat returns a response from Lewis (not an error)
6. Response reflects Lewis's SOUL.md personality
7. Refreshing the page shows the previous conversation
8. All 344 existing tests still pass

## Rollback Plan

All changes on a feature branch (`feat/chat-mvp`). If anything breaks, reset to `main`. The `~/.fort` directory is new and can be deleted without affecting anything.

## Open Questions

None — this is scoped to local functionality only. No deployment, no external integrations.
