# SPEC-011: Tool Approval UI

**Status:** Draft  
**Author:** Lewis  
**Date:** 2026-03-25  
**Priority:** HIGH — user safety  
**Depends on:** Spec 002 (Tool Execution), Spec 009 (Notifications)

---

## Problem

Tier 2 and Tier 3 tools require user approval before execution, but the current implementation blocks the agent waiting for a WS message that the user must manually construct. There is no proper UI for reviewing and approving/rejecting pending tool calls. The agent silently waits with no visible queue.

## Goals

1. Dashboard shows all pending tool approvals prominently
2. User can review: tool name, parameters, which agent/task is requesting
3. Approve or reject with one click (+ optional rejection reason)
4. Approval queue updates in real-time via WebSocket
5. Tier 3 tools show a stronger warning (destructive/irreversible)
6. Approved/rejected history visible in task detail

## Non-Goals (v1)

- Approval delegation to other users
- Time-limited auto-approve rules ("always allow write-file for Pascal")
- Mobile push for approval requests

---

## Design

### Approval Queue (in-memory + DB)

When an agent requests a Tier 2/3 tool:
1. ToolExecutor creates an `ApprovalRequest` record in DB
2. Publishes `approval.required` on the bus (triggers notification via Spec 009)
3. Pauses execution — `completeWithTools()` awaits resolution
4. Resolution arrives via `approval.respond` WS message or timeout

```sql
CREATE TABLE approval_requests (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  tool_name TEXT NOT NULL,
  tool_tier INTEGER NOT NULL,
  parameters TEXT NOT NULL,   -- JSON
  status TEXT DEFAULT 'pending',  -- 'pending' | 'approved' | 'rejected'
  rejection_reason TEXT,
  created_at TEXT DEFAULT (datetime('now')),
  resolved_at TEXT
);
```

### ApprovalStore

File: `packages/core/src/tools/approval-store.ts`

```typescript
export class ApprovalStore {
  constructor(db: Database.Database) {}
  initSchema(): void
  create(request: CreateApprovalInput): ApprovalRequest
  resolve(id: string, approved: boolean, rejectionReason?: string): ApprovalRequest
  getPending(): ApprovalRequest[]
  getForTask(taskId: string): ApprovalRequest[]
}
```

### ToolExecutor Changes

ToolExecutor gains an optional `approvalStore` + `bus` + `awaitApproval` callback:

```typescript
// When tier >= 2, before executing:
const approval = approvalStore.create({ taskId, agentId, toolName, tier, parameters });
bus.publish('approval.required', agentId, { approvalId: approval.id, ... });
const result = await awaitApproval(approval.id, timeout: 600000); // 10 min timeout
if (!result.approved) throw new Error(`Rejected: ${result.rejectionReason}`);
// proceed with execution
```

### WebSocket Handlers

```
'approvals.list'      → get all pending approvals
'approval.respond'    → { id, approved, rejectionReason? } → resolves the pending await
'approvals.for_task'  → get approvals for a specific task (history)
```

### Dashboard: Approval Center

1. **Header badge**: if any pending approvals, show count badge on a shield icon (distinct from notification bell)
2. **Approvals panel** (click badge or /approvals route):
   - List of pending approvals sorted by time
   - Each card: agent avatar, tool name, tier badge (⚠️ Tier 2 / 🔴 Tier 3 DESTRUCTIVE), task title, parameters (formatted JSON), timestamp
   - Approve button (green), Reject button (red) with reason input
3. **Task detail** includes approval history section

### Tier Badges

- Tier 2: amber warning icon "Requires approval"
- Tier 3: red danger icon "DESTRUCTIVE / IRREVERSIBLE — Review carefully"
  
Parameters display: syntax-highlighted JSON with key names bolded.

---

## Test Criteria

- ToolExecutor creates ApprovalRequest for Tier 2 tool
- ToolExecutor blocks until approval resolved
- approval.respond WS message resolves the await
- Rejection throws error, task marked failed
- Timeout (10 min) rejects automatically
- Approval history visible in task detail
- All existing 510+ tests still pass

---

## Rollback

Approval flow gated by tier. Tier 1 tools bypass entirely — zero regression risk. If ApprovalStore not initialized, ToolExecutor falls back to auto-approve (dev mode behavior, matching current behavior).
