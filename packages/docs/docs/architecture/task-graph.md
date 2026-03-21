---
sidebar_position: 3
title: TaskGraph
---

# TaskGraph -- Atomic Unit of Transparency

The core invariant of Fort: **every conversation creates a task.** Tasks are the atomic unit of transparency. If Fort is doing work, there is a task tracking it. If there is no task, no work is happening.

## Task Structure

```typescript
interface Task {
  id: string;                    // UUID v4
  parentId: string | null;       // for subtask hierarchies
  title: string;
  description: string;
  status: TaskStatus;
  source: TaskSource;            // 'user_chat' | 'scheduled' | 'agent_delegation' | ...
  assignedAgent: string | null;
  createdAt: Date;
  updatedAt: Date;
  completedAt: Date | null;
  metadata: Record<string, unknown>;
  subtaskIds: string[];
  threadId: string | null;
}

type TaskStatus = 'created' | 'in_progress' | 'blocked'
               | 'needs_review' | 'completed' | 'failed';
```

## Creating Tasks

Every entry point into Fort creates a task. A user message through `fort.chat()`, a scheduled routine, or an agent delegating work -- all go through `createTask`:

```typescript
const task = taskGraph.createTask({
  title: 'Summarize open PRs',
  source: 'user_chat',
  assignedAgent: 'orchestrator',
});
```

The TaskGraph publishes `task.created` on the bus. The Orchestrator subscribes and begins work.

## Task Decomposition

Complex tasks break down into subtasks. The `decompose` method creates child tasks linked to a parent:

```typescript
const subtasks = taskGraph.decompose(parentTask.id, [
  { title: 'Fetch PR list', assignedAgent: 'memory' },
  { title: 'Generate summary', assignedAgent: 'orchestrator' },
]);
```

The parent moves to `in_progress`. When all children reach a terminal state (`completed` or `failed`), the parent auto-transitions:

- All children completed --> parent marked `completed`
- Any child failed --> parent marked `needs_review`

## Status Transitions

Status changes are tracked with timestamps and optional reasons:

```typescript
taskGraph.updateStatus(taskId, 'in_progress');
taskGraph.updateStatus(taskId, 'blocked', 'Waiting for API rate limit');
taskGraph.updateStatus(taskId, 'completed', 'All subtasks finished');
```

Every transition publishes `task.status_changed` with the previous status, new status, and reason. The IPC server picks these up and broadcasts them to the Swift menu bar and Tauri dashboard in real time.

## Agent Assignment

Tasks can be reassigned at any time. This publishes `task.assigned`:

```typescript
taskGraph.assignAgent(taskId, 'reflection');
```

## Querying Tasks

The TaskGraph supports filtered queries and convenience methods:

```typescript
// Filtered query
const myTasks = taskGraph.queryTasks({
  status: 'in_progress',
  assignedAgent: 'orchestrator',
  search: 'deploy',
});

// Convenience methods
taskGraph.getActiveTasks();     // status === 'in_progress'
taskGraph.getBlockedTasks();    // status === 'blocked'
taskGraph.getAllTasks();         // sorted by creation time, newest first
taskGraph.getTaskCount();       // total count
```

## Stale Task Detection

Tasks that have not been updated for a configurable duration (default 30 minutes) are considered stale:

```typescript
const stale = taskGraph.getStaleTasks();           // default 30 min
const veryStale = taskGraph.getStaleTasks(60 * 60 * 1000); // 1 hour
```

`fort doctor` reports stale tasks as a `degraded` health status. The Reflection agent periodically checks for stale tasks and either nudges the assigned agent or escalates to the Orchestrator.

## Threads

Threads are named conversation contexts backed by a task. A thread groups related interactions:

```typescript
const thread = taskGraph.createThread({
  name: 'Deploy v2.1',
  taskId: task.id,
});

taskGraph.updateThreadStatus(thread.id, 'paused');
```

Threads publish `thread.created` and `thread.status_changed` events. The ThreadManager module (separate from TaskGraph) handles persistence and richer thread operations like branching and project tagging.

## Diagnostics

The task diagnostic provider reports total count, active count, blocked count, and stale count. Any stale tasks trigger a `degraded` status in `fort doctor` output.
