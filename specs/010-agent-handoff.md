# SPEC-010: Agent Handoff & Inter-Agent Messaging

**Status:** Draft  
**Author:** Lewis  
**Date:** 2026-03-25  
**Priority:** HIGH — core multi-agent capability  
**Depends on:** Spec 004 (Agent Management), Spec 005 (Task Persistence)

---

## Problem

Fort has multiple specialist agents (researcher, coder, analyst, etc.) but they cannot collaborate. When a task is too large or cross-domain, there is no mechanism for one agent to:
- Delegate a subtask to a more capable agent
- Pass context to another agent
- Receive results back and continue

Each agent works in isolation. This is a fundamental limitation for multi-agent workflows.

## Goals

1. Orchestrator agent can delegate subtasks to specialist agents
2. Specialist agents can request help from other agents
3. Handoff preserves context (parent task ID, conversation thread)
4. Dashboard shows agent-to-agent delegation visually (task tree)
5. No deadlock — circular delegation detected and rejected

## Non-Goals (v1)

- Streaming results between agents
- Parallel multi-agent execution (sequential only)
- Agent voting / consensus mechanisms
- Cross-Fort agent communication

---

## Design

### Handoff Model

A "handoff" is a subtask created by one agent (`sourceAgentId`) assigned to another (`targetAgentId`). The parent task is paused while the subtask runs. When the subtask completes, its result is injected into the parent agent's conversation as a system message.

### Task Changes

Add to the `tasks` table (already exists from Spec 005):
- `source_agent_id TEXT` — which agent created this subtask
- `parent_task_id TEXT` — already exists, but now used for handoff chains

### AgentHandoff Tool (Tier 1)

A new built-in tool available to all agents: `delegate-to-agent`

```typescript
{
  name: 'delegate-to-agent',
  description: 'Delegate a subtask to a specialist agent',
  tier: 1,  // no approval required — agent can delegate freely
  parameters: {
    agentId: string,       // target agent ID
    taskTitle: string,     // title for the delegated task
    taskDescription: string, // full context + what you need
    expectation: string,   // what result format you expect back
  }
}
```

When this tool is called:
1. Creates a new Task with `parentId` = current task ID, `assignedAgent` = target
2. Publishes `task.created` for the target agent
3. The calling agent's `completeWithTools()` loop pauses waiting for `task.completed` on the subtask
4. When subtask completes, result is returned as the tool response
5. Calling agent continues with the result

### Circular Delegation Detection

Before creating a handoff, walk up the parent_task chain. If `targetAgentId` appears anywhere in the chain, reject with an error message explaining the cycle.

Max depth: 5 levels of delegation. Beyond that, reject with "delegation chain too deep."

### Orchestrator Changes

The Orchestrator (`services/orchestrator.ts`) gains awareness of which agent is assigned to each task. When a `task.created` event fires, route to the correct agent rather than the default.

### Dashboard: Task Tree View

In the Tasks page (from Spec 005), show delegated subtasks as a collapsible tree under the parent task:

```
📋 Research cloud pricing for Q2 budget       [Lewis]     completed
  └─ 📋 Pull pricing from AWS pricing API     [Pascal]    completed  
  └─ 📋 Calculate 3-year TCO scenarios        [Milton]    completed
```

Task detail panel shows `delegated by: {agent name}` when applicable.

### WebSocket Handlers

No new handlers needed — uses existing `tasks.query` from Spec 005. Add `subtasks` field to task response that includes child tasks.

---

## Deadlock Prevention

- Max delegation depth: 5
- Circular agent detection: walk parent chain
- Timeout: subtask must complete within 10 minutes or parent gets a timeout result
- If target agent is stopped: return error immediately, don't block

---

## Test Criteria

- Orchestrator routes delegated task to correct agent
- Parent task pauses while subtask runs
- Subtask result returned to parent as tool response
- Circular delegation detected and rejected
- Max depth enforced
- Task tree visible in queryTasks (subtasks array)
- All existing 510 tests still pass

---

## Migration

No DB migration needed — `parent_task_id` already exists in Spec 005 schema. Add `source_agent_id` as ALTER TABLE (SQLite supports ADD COLUMN).
