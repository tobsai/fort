# Fort — Architectural Review
**Reviewer:** Marty (Product Owner)  
**Date:** 2026-03-25  
**Tone:** Honest. Constructive. No punches pulled.

---

## Summary Verdict

Fort has a solid bones: typed event bus, task-centric design, clean module boundaries on paper. But the codebase is carrying structural debt that will compound fast. The server is doing too much, the Fort class is a god object by another name, the task graph has no persistence, and the auth implementation is functional-but-fragile. These aren't nitpicks — they're the kind of decisions that get expensive to reverse.

---

## 1. The God Object Problem (`fort.ts`)

**Verdict: This IS a god object. The `Fort` class has 25+ public properties.**

Every module — memory, permissions, tools, scheduler, specs, tokens, behaviors, routines, flags, plugins, harness, gc, rewind, threads, llm, introspect, osIntegration, ipc, doctor — is a public field on `Fort`. That means `FortServer` can reach in and call `this.fort.memory.search(...)` directly. That's not encapsulation; that's a bag of globals with a class wrapper.

**The real question:** Why does `FortServer` need `this.fort.llm`, `this.fort.agentFactory`, `this.fort.taskGraph`, and `this.fort.agents` all directly? If Fort had a proper facade pattern (`fort.chat()`, `fort.status()`, `fort.agents()`) the server wouldn't need to know about internals.

`fort.chat()` is exactly right — a narrow public API. There should be more of those and fewer raw module references.

**Recommendation:** Create a `FortPortal` interface that exposes only what the server needs. The full `Fort` class stays internal. This costs ~2 hours now and saves you from sprawl later.

---

## 2. The Server Is Doing Too Much (`server/index.ts`)

`FortServer` is a WebSocket server, an HTTP file server, an auth handler, a REST API router, and an agent creation wizard all in one 500+ line file. This is a maintenance problem.

**Specific issues:**

- **Inline `require()` calls** inside request handlers (`require('node:fs')`, `require('node:os')`, `require('node:path')`). These are at module scope already. Inline requires inside request handlers are a code smell — they're synchronous module lookups inside hot paths.

- **The `handleMessage` switch block** handles `chat`, `status`, `tasks`, `tasks.active`, `agents`, `memory.search`, `memory.stats`, `doctor`. This is a WebSocket RPC handler — it belongs in its own class, not bolted onto the HTTP server.

- **Agent creation via REST** (`/api/agents/create`) is a multi-step async operation with business logic inside an HTTP handler. The `handleCreateAgent` method writes SOUL.md, saves identity YAML, copies avatar files, and refreshes the agent — that's domain logic living in a transport layer.

- **Static file serving** is implemented from scratch when Express, Fastify, or even a 10-line helper would do. The SPA fallback logic is fine, but it's unnecessary code to maintain.

**Recommendation:** Split into `HttpRouter`, `WebSocketHandler`, and keep `FortServer` as a thin coordinator. Agent creation logic belongs in `AgentFactory`, not the server.

---

## 3. The Task Graph Has No Persistence (`task-graph/index.ts`)

**This is the biggest architectural risk in the project.**

Everything is in a `Map<string, Task>`. Restart the process, lose every task. The task graph is described as "the atomic unit of transparency" — but it has zero durability. This isn't an MVP tradeoff, it's a fundamental contradiction of the stated design principle.

The code already uses `better-sqlite3` for memory, tokens, tools, flags, rewind, and threads. Adding SQLite persistence to TaskGraph is not a big lift — but it becomes exponentially harder once the in-memory graph has callers in 20+ places relying on synchronous `Map` lookups.

**The LLM completion review in task-graph** (`reviewCompletion()`) is also concerning. The task graph is calling the LLM directly. That's business logic inside infrastructure. The task graph should be dumb state — status transitions, parent/child relationships. The review logic belongs in `OrchestratorService` or a dedicated `CompletionReviewer`.

**The `detectInability()` method** is a 25-pattern regex list living in the task graph. This is clearly wrong. It's an LLM response parsing concern and should be in the LLM layer or orchestrator.

**Recommendation:** SQLite-back the task graph. Move `reviewCompletion()` and `detectInability()` to the orchestrator. Do this before you add any more features.

---

## 4. The Orchestrator Isn't Really Orchestrating

`OrchestratorService.routeChat()` does three things: finds a default agent, creates a task, calls `agent.handleTask()`. That's a router, not an orchestrator. The word "orchestrate" implies coordination across multiple agents, subtask delegation, retry logic, fallback routing. None of that exists.

**More critically:** There's no retry logic anywhere in the request path. If `agent.handleTask()` throws, the task stays `in_progress` forever (or crashes the promise chain). There's no timeout. There's no fallback agent. There's no dead-letter queue.

The orchestrator also has a `checkStaleTasks()` method that isn't wired to anything visible — it publishes events but nothing appears to call it on a schedule.

**Recommendation:** Wire stale task checking to the Scheduler. Add a try/catch with task failure on `routeChat`. Even a 30-second timeout per task would be a meaningful safety net.

---

## 5. Auth: Functional but Fragile (`server/auth.ts`)

The implementation is actually cleaner than expected — HMAC-signed cookies, timing-safe comparison, proper cookie flags. Credit where due.

**However:**

- **No CSRF protection.** The `/auth/google/callback` route processes `GET` requests with sensitive OAuth codes. Anyone who tricks a user into visiting a crafted URL could potentially hijack the auth flow. The state parameter (CSRF token) is missing from the OAuth flow entirely.

- **`SESSION_SECRET` defaults to empty string.** `process.env.SESSION_SECRET ?? ''`. If `SESSION_SECRET` is not set in production, every session is signed with an empty key. This is silent and catastrophic. It should throw at startup, not silently default.

- **The WebSocket auth check** (`verifyClient`) only runs at connection time. If a cookie expires mid-session, the WebSocket stays open. Fine for a personal tool, worth noting for a multi-user deployment.

- **`FORT_AUTH_ENABLED !== 'false'`** — auth is ON by default (good), but the check is a string comparison on a negated condition, which is the most fragile way to parse an env boolean. `FORT_AUTH_ENABLED=0` would not disable auth.

- **`ACCESS_DENIED` returns raw HTML** in the callback handler. The rest of the app returns JSON. Pick one.

**Recommendation:** Add OAuth state parameter for CSRF. Throw at startup if `SESSION_SECRET` is empty and auth is enabled. These are real security gaps, not theoretical ones.

---

## 6. The Monorepo Structure: Premature?

Five packages: `core`, `cli`, `dashboard`, `swift-shell`, `docs`. For a project at v0.1.0 with one developer, this is overhead.

The `dashboard` package is compiled separately and then physically `COPY`ed into the Docker image. The server resolves its path via `__dirname` traversal (`join(__dirname, '..', '..', '..', 'dashboard', 'dist')`). That's a path-coupling hack that will silently break if either package moves.

`swift-shell` exists as a package in a Node.js monorepo. Swift doesn't participate in `npm workspaces`. It's just a directory. That's fine, but it shouldn't be listed as a workspace — it creates confusion.

The `docs` package is not in the root `package.json` workspaces at all (only `core`, `cli`, `dashboard` are listed). So why does it exist as a `packages/` directory?

**Recommendation:** `core` + `cli` in one package until there's a real reason to split. The dashboard should be a Vite project at the root level until the monorepo earns its complexity.

---

## 7. Dashboard: Properly Separated? Sort Of.

Three pages, a few components. The React app communicates exclusively via WebSocket — no REST (except initial loads from HTTP endpoints). That's actually a clean decision.

**Concerns:**

- There are no React error boundaries anywhere in `App.tsx` or the pages. A single component throw will blank the entire UI.

- The dashboard is served from the same process as the AI runtime. If an agent crashes the Node process, the dashboard goes dark too. For a personal tool, acceptable. For a multi-user deployment, this needs a separate process.

- The `App.tsx` has no loading state. The app navigates to `/chat` immediately — if the WebSocket hasn't connected yet, the user sees... whatever the initial render is.

---

## 8. Test Coverage: Quantity Over Quality?

~7,000 lines of tests across 24 files. That's respectable in volume. But:

- `fort.test.ts` clears the `ANTHROPIC_API_KEY` to prevent real LLM calls. This means the tests never exercise the actual LLM integration path. The most important code path — agent receives task, calls LLM, gets result — has no real integration test.

- The `task-graph.test.ts` presumably tests the in-memory map. When you add SQLite persistence, all those tests will need to be rewritten or extended.

- `flows.test.ts` at 453 lines is the most promising file — but "flows" testing an in-memory system isn't the same as testing a durable system under restart.

**The missing tests:**
  - Server auth bypass attempt (no CSRF state)
  - Task persistence across restart
  - Agent failure → task failure path
  - WebSocket reconnection behavior
  - Rate limiting on the LLM client

---

## 9. Missing Concerns

| Concern | Status |
|---|---|
| Rate limiting on LLM calls | ❌ Not found |
| Token budget enforcement | Tracked but not enforced as a hard limit |
| Agent output sanitization (XSS in dashboard) | Not visible — dashboard renders agent output, needs checking |
| Retry logic on LLM API failures | ❌ Not found |
| Request timeout on `handleTask` | ❌ Not found |
| Process memory limit (unbounded task Map) | ❌ Not found |
| Healthcheck returns agent count | ✅ Fine |
| OAuth CSRF (state param) | ❌ Missing |
| Empty SESSION_SECRET check | ❌ Missing |

---

## 10. What's Actually Good

- **The event bus design** is clean. Typed events, subscriber pattern, no circular dependencies between modules.
- **`ToolExecutor` with tier-gated permissions** is a thoughtful design.
- **The `detectInability()` pattern** — wrong layer, but the idea is right. Agents should be held accountable for not just saying "I can't do that."
- **Auth cookie implementation** — HMAC signing, timing-safe comparison, HttpOnly + SameSite flags. Whoever wrote this actually read the security docs.
- **Dockerfile multi-stage build** is clean and correct.
- **`railway.json` healthcheck** is properly configured.
- **The spec-driven development directive** in CLAUDE.md is the right instinct. Stick to it.

---

## Priority Fixes (in order)

1. **SESSION_SECRET empty check** — throw at startup, not silent fail. One line fix, real security impact.
2. **OAuth state parameter** — CSRF on the auth callback. This is a known OAuth 2.0 requirement.
3. **Task graph persistence** — SQLite. Before adding more features that assume durability.
4. **`reviewCompletion()` / `detectInability()` move** — out of task graph, into orchestrator.
5. **Agent task failure path** — try/catch in `routeChat`, mark task failed on exception.
6. **React error boundaries** — one at app root, one per page.
7. **Fort facade interface** — narrow the surface area the server can access.

---

*The foundation is buildable. But some of these debt items have a multiplying effect — the longer task persistence waits, the more code assumes in-memory behavior. Fix the data layer before the feature layer grows.*
