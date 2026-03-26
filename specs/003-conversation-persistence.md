# Conversation Persistence — Wire ThreadManager into Chat Flow

**ID:** SPEC-003
**Status:** draft
**Author:** Pascal (dev agent)
**Created:** 2026-03-25
**Depends on:** SPEC-001 (Chat MVP), SPEC-002 (Tool Execution)

## Goal

Every chat message sent through Fort is persisted in a named thread backed by SQLite, so
conversation history survives page refreshes, server restarts, and agent switches.
The `ThreadManager` already exists with full SQLite persistence — this spec wires it into
the WebSocket chat flow and exposes thread operations to the dashboard.

## Current State

- `ThreadManager` (`packages/core/src/threads/index.ts`) is fully built: SQLite-backed threads,
  messages, forking, cross-references, search, diagnostics. 448 tests pass.
- `FortServer` handles `'chat'` WebSocket messages by calling `fort.chat()` →
  `OrchestratorService.routeChat()` → task created → agent runs. Messages are **not** persisted.
- `ChatPage.tsx` loads history from `GET /api/chat-history` (task-based, in-memory volatile store).
- No WebSocket handlers exist for thread operations.

**What's missing:**
1. Thread created/found on first chat message to an agent
2. User message saved to thread before routing to agent
3. Agent response saved to thread when task completes
4. WebSocket handlers: `threads.list`, `thread.history`, `thread.create`
5. Dashboard loads history from threads instead of task list

## Design Principles

- **One active thread per agent** — the default conversation thread for each agent. Named threads
  (via `thread.create`) allow topic isolation in future.
- **Server-side persistence** — thread writes happen in `FortServer`, not in the agent or
  dashboard. The dashboard is a pure consumer.
- **Non-breaking** — the existing task-based `GET /api/chat-history` endpoint is kept for
  backward compatibility. New thread-based history is additive.
- **Greeting messages are not persisted** — `__greeting__` internal messages are implementation
  details, not user content.

## Approach

### Phase 1: Server-side Thread Wiring (`packages/core/src/server/index.ts`)

Add two private maps to `FortServer`:
- `taskToThread: Map<string, string>` — maps in-flight task IDs to their thread ID so the
  `task.status_changed` bus event can persist the agent response
- No in-memory thread cache needed; `ThreadManager.listThreads({ agentId })` is fast (indexed)

**In `handleMessage` `'chat'` case:**
1. If not a greeting, call `getOrCreateAgentThread(agentId)` to find the active thread
2. Persist the user message: `fort.threads.addMessage(threadId, { role: 'user', content })`
3. Store `taskToThread.set(task.id, threadId)` after `fort.chat()` returns
4. Return response as before (no change to `chat.response` shape)

**In `setupEventBroadcast` — new `task.status_changed` persistence handler:**
```
On task.status_changed:
  if task.result && taskToThread.has(task.id):
    fort.threads.addMessage(threadId, { role: 'agent', content: task.result, agentId })
    taskToThread.delete(task.id)
```

**New WebSocket message types:**

| Type | Payload | Response |
|------|---------|----------|
| `threads.list` | `{ agentId?: string }` | `threads.list.response` → `{ threads: Thread[] }` |
| `thread.history` | `{ agentId?: string; threadId?: string; limit?: number }` | `thread.history.response` → `{ threadId, agentId?, messages: ThreadMessage[] }` |
| `thread.create` | `{ name: string; agentId?: string; description?: string }` | `thread.create.response` → `{ thread: Thread }` |

**`getOrCreateAgentThread(agentId)`** private method:
- Lists active threads for `agentId`
- Returns existing thread ID if found
- Creates new thread `"Chat"` with `assignedAgent: agentId` if none

### Phase 2: Tests (`packages/core/src/__tests__/conversation-persistence.test.ts`)

Unit tests using real `ThreadManager` + `ModuleBus` + `TaskGraph` (same pattern as threads.test.ts):
1. `getOrCreateAgentThread` returns existing thread on second call
2. User message is persisted before agent handles the task
3. Agent response is persisted on `task.status_changed`
4. `threads.list` returns threads filtered by agentId
5. `thread.history` returns messages for a thread
6. `thread.create` creates a named thread
7. Greeting messages are NOT persisted

### Phase 3: Dashboard Types (`packages/dashboard/src/types/index.ts`)

Add:
```typescript
export interface Thread {
  id: string;
  name: string;
  assignedAgent: string | null;
  status: 'active' | 'paused' | 'resolved';
  lastActiveAt: string;
  createdAt: string;
}

export interface ThreadMessage {
  id: string;
  threadId: string;
  role: 'user' | 'agent' | 'system';
  content: string;
  agentId: string | null;
  createdAt: string;
}
```

### Phase 4: Dashboard ChatPage (`packages/dashboard/src/pages/ChatPage.tsx`)

**Replace `fetchChatHistory()` with thread-based loading:**
1. When `agentId` is selected (or changes), send `thread.history` with `{ agentId }`
2. Subscribe to `thread.history.response` — convert `ThreadMessage[]` to `ChatMessage[]`
3. Set `historyFetched = true` after receiving response (or if `messages` is empty)
4. Store `threadId` per agent in `localStorage` so it survives refreshes (future fork support)

**Convert `ThreadMessage` → `ChatMessage`:**
- `role: 'user'` → `{ role: 'user', text: content, ts: Date.parse(createdAt) }`
- `role: 'agent'` → `{ role: 'agent', text: content, ts: Date.parse(createdAt) }`
- `role: 'system'` → skip

## Affected Files

**New files:**
- `specs/003-conversation-persistence.md` (this file)
- `packages/core/src/__tests__/conversation-persistence.test.ts`

**Modified files:**
- `packages/core/src/server/index.ts` — thread wiring + new WS handlers
- `packages/dashboard/src/types/index.ts` — Thread, ThreadMessage types
- `packages/dashboard/src/pages/ChatPage.tsx` — thread-based history loading

## Test Criteria

1. Sending a chat message creates a thread for the agent (if none exists)
2. Second chat to the same agent reuses the same thread
3. User message appears in `thread.history` for that agent
4. Agent response appears in `thread.history` after task completes
5. Greeting (`__greeting__`) messages are NOT persisted
6. `threads.list` returns correct threads filtered by agentId
7. `thread.create` creates a named thread and returns it
8. Dashboard loads thread history on agent select (no REST API call needed)
9. History survives a dashboard refresh (messages retrieved from SQLite)
10. All existing 448 tests still pass

## Rollback Plan

Feature branch (`feat/conversation-persistence`). All changes are additive:
- `GET /api/chat-history` REST endpoint is unchanged
- Thread persistence is fire-and-forget (errors are caught, chat never fails)
- Dashboard falls back to empty history if thread history is unavailable

## Open Questions

1. **Multiple threads per agent** — Future: allow `thread.create` to start a new named thread,
   with agent selector updated to show thread list. Out of scope for this spec.
2. **History pagination** — `thread.history` accepts `limit` but the dashboard doesn't paginate yet.
   Default limit: 200 messages.
3. **Tool call messages** — Tool events are not persisted in threads (they're ephemeral events).
   Only user/agent text is stored.
