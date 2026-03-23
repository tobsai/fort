---
sidebar_position: 4
title: WebSocket IPC
---

# WebSocket IPC Server

The IPC server bridges the TypeScript core to native clients -- the Swift menu bar app and the Tauri dashboard. It runs on `ws://localhost:4001/shell` and provides real-time state broadcasting plus a request/response action protocol.

## Startup

IPC starts as part of `fort.start()` but is **non-fatal**. If port 4001 is already in use (another Fort instance, a test runner), startup continues without IPC. No functionality is lost -- the CLI still works. Native clients simply cannot connect until the port is free.

```typescript
// Inside Fort.start():
try {
  await this.ipc.start();
} catch {
  // Port may be in use -- continue without IPC
}
```

## Message Format

All messages between server and clients are JSON objects with `type` and `data` fields:

```typescript
interface IPCMessage {
  type: string;       // 'status' | 'tasks' | 'agents' | 'notification' | 'error' | ...
  data: unknown;
}
```

Client-to-server messages use an action format:

```typescript
interface IPCAction {
  action: string;
  payload?: Record<string, unknown>;
}
```

## Supported Actions

Clients send actions; the server responds with a typed message.

| Action | Payload | Response Type | Description |
|--------|---------|---------------|-------------|
| `get_status` | -- | `status` | Agent health summary with green/yellow/red indicator |
| `get_tasks` | -- | `tasks` | Active, queued, and completed task counts |
| `get_agents` | -- | `agents` | Full agent list with status, capabilities, task/error counts |
| `run_doctor` | -- | `doctor` | Run all diagnostic providers and return results |
| `run_routine` | `{ routineId }` | `routine_result` | Execute a named routine |
| `spotlight_query` | `{ query }` | `spotlight_results` | Search Fort from macOS Spotlight |
| `shortcut_action` | `{ intent, params }` | `shortcut_result` | Handle a macOS Shortcut invocation |
| `file_action` | `{ filePaths }` | `file_action_result` | Process files dropped onto Fort |
| `voice_input` | `{ transcript }` | `voice_result` | Handle transcribed voice input |
| `notification_policy` | `{ category, focusMode }` | `notification_policy` | Query notification rules for a category |

## Real-Time Broadcasting

The IPC server subscribes to ModuleBus events and broadcasts state updates to all connected clients automatically:

**Task events** (`task.created`, `task.updated`, `task.completed`, `task.failed`) trigger a `tasks` broadcast with updated counts.

**Agent events** (`agent.started`, `agent.stopped`, `agent.paused`, `agent.error`) trigger a `status` broadcast with the health indicator.

**Notification events** -- task completion, task failure, agent creation, and flow completion publish `notification` messages that native clients can surface as macOS notifications:

```json
{
  "type": "notification",
  "data": {
    "title": "Task Completed",
    "body": "Summarize open PRs",
    "category": "task"
  }
}
```

## Client Tracking

The server tracks each connected client with:

- **Client ID** -- monotonically increasing integer
- **Connection time** -- when the client connected
- **Messages sent/received** -- per-client counters

These metrics are surfaced through `diagnose()` and included in the `fort doctor` output.

## Diagnostics

The IPC diagnostic provider reports:

- Server status (listening or not)
- Connected client count with per-client details
- Uptime in seconds
- Total message throughput (sent and received)

```typescript
const diag = ipc.diagnose();
// { module: 'ipc', status: 'healthy', checks: [...] }
```

## Architecture Notes

- The server uses the `ws` library (not Socket.IO) for a minimal dependency footprint.
- All broadcasts are fire-and-forget. Clients that disconnect mid-broadcast are cleaned up on the next `close` or `error` event.
- The `/shell` path is intentional -- it keeps the door open for additional WebSocket endpoints (e.g., `/stream` for token streaming) without path collisions.
