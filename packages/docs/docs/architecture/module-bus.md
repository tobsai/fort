---
sidebar_position: 2
title: ModuleBus
---

# ModuleBus -- Event-Driven Backbone

The ModuleBus is Fort's central nervous system. Every module publishes and subscribes to typed events through this single bus. There are no direct imports between peer modules -- events are the only communication channel.

## Why Events?

Fort replaced OpenClaw's brittle heartbeat/cron system with clear, inspectable event streams. The bus gives you:

- **Decoupling.** Modules know nothing about each other. The TaskGraph publishes `task.created`; the IPC server, the Orchestrator, and the TokenTracker can all react independently.
- **Inspectability.** Every event is recorded in a ring buffer (default 1000 entries). You can query the history at any time.
- **Error isolation.** A failing handler does not crash other subscribers. Errors are caught and re-published as `bus.error` events.

## Event Format

Every event on the bus follows this shape:

```typescript
interface FortEvent<T = unknown> {
  id: string;        // UUID v4
  type: string;      // dot-namespaced, e.g. "task.created"
  source: string;    // module that published, e.g. "task-graph"
  timestamp: Date;
  payload: T;
}
```

## API

### subscribe

Register a handler for a specific event type. Returns an unsubscribe function.

```typescript
const unsub = bus.subscribe<{ task: Task }>('task.created', (event) => {
  console.log('New task:', event.payload.task.title);
});

// Later:
unsub();
```

### publish

Emit a typed event. Handlers run sequentially; errors are caught per-handler and published as `bus.error`.

```typescript
await bus.publish('task.created', 'task-graph', { task });
```

### getHistory

Retrieve past events, optionally filtered by type and limited in count.

```typescript
const recent = bus.getHistory('task.created', 10);
```

### Utilities

```typescript
bus.getSubscriptionCount();          // total handlers across all types
bus.getSubscriptionCount('task.*');   // handlers for one type
bus.getEventTypes();                 // all types with active subscribers
bus.clear();                         // reset handlers and history
```

## Common Event Types

| Namespace   | Events |
|-------------|--------|
| `task.*`    | `task.created`, `task.status_changed`, `task.assigned`, `task.completed`, `task.failed` |
| `agent.*`   | `agent.started`, `agent.stopped`, `agent.paused`, `agent.error`, `agent.hatched` |
| `thread.*`  | `thread.created`, `thread.status_changed` |
| `flow.*`    | `flow.started`, `flow.completed`, `flow.step_completed` |
| `llm.*`     | `llm.request`, `llm.response`, `llm.error` |
| `tokens.*`  | `tokens.recorded` |
| `rewind.*`  | `rewind.snapshot`, `rewind.restored` |
| `fort.*`    | `fort.started`, `fort.stopped` |
| `bus.*`     | `bus.error` (internal) |

## Error Handling

When a subscriber throws, the bus catches the error, continues running remaining handlers, and publishes a `bus.error` event with the original event type and error messages. To prevent infinite loops, errors inside `bus.error` handlers are silently swallowed.

```typescript
bus.subscribe('bus.error', (event) => {
  const { originalEvent, errors } = event.payload;
  logger.warn(`Handler errors on ${originalEvent}:`, errors);
});
```

## Design Constraints

- **No wildcards.** Subscriptions are exact-match on event type. If you need to react to all task events, subscribe to each one individually. This keeps routing predictable.
- **Sequential execution.** Handlers for one event type run in registration order, awaited one at a time. This guarantees ordering within a type but means slow handlers delay subsequent ones.
- **Ring buffer history.** The bus retains the last `maxHistory` events (default 1000). Older events are dropped. For durable event logs, use the Rewind module.
