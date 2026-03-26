# SPEC-007: Model Cost Tracking

**Status:** Draft  
**Author:** Lewis  
**Date:** 2026-03-25  
**Depends on:** Spec 005, Spec 006

---

## Problem

There is no visibility into how many tokens Fort uses or what it costs. When running multiple agents on Anthropic's Max plan, token usage isn't a billing concern — but it IS a signal for:
- Which agents are expensive to run
- Which tasks are disproportionately token-heavy
- Whether agents are going into unnecessary loops
- Capacity planning for when Fort runs on a paid API

## Goals

1. Track tokens used per task (input + output + cache_read + cache_write)
2. Track tokens per agent over time
3. Show usage in the dashboard (agent cards + task detail)
4. Aggregate daily/weekly summaries
5. Cost estimation (configurable $ per 1M tokens per model)

## Non-Goals (v1)

- Hard budget limits / kill switches
- Per-user billing
- Real-time cost alerts

---

## Design

### Data Model

```sql
CREATE TABLE usage_events (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL,           -- FK to tasks table
  agent_id TEXT NOT NULL,
  model TEXT NOT NULL,
  input_tokens INTEGER DEFAULT 0,
  output_tokens INTEGER DEFAULT 0,
  cache_read_tokens INTEGER DEFAULT 0,
  cache_write_tokens INTEGER DEFAULT 0,
  total_tokens INTEGER GENERATED ALWAYS AS (input_tokens + output_tokens + cache_read_tokens + cache_write_tokens) STORED,
  estimated_cost_usd REAL,         -- calculated at insert time
  created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX idx_usage_agent ON usage_events(agent_id);
CREATE INDEX idx_usage_task ON usage_events(task_id);
CREATE INDEX idx_usage_date ON usage_events(created_at);

CREATE TABLE model_pricing (
  model TEXT PRIMARY KEY,
  input_per_million REAL NOT NULL,
  output_per_million REAL NOT NULL,
  cache_read_per_million REAL DEFAULT 0,
  cache_write_per_million REAL DEFAULT 0,
  updated_at TEXT DEFAULT (datetime('now'))
);
```

### Default Pricing (seeded at startup)

| Model | Input/M | Output/M |
|---|---|---|
| claude-opus-4-5 | $15.00 | $75.00 |
| claude-sonnet-4-5 | $3.00 | $15.00 |
| claude-haiku-4-5 | $0.80 | $4.00 |
| gpt-4o | $5.00 | $15.00 |
| gpt-4o-mini | $0.15 | $0.60 |
| llama-3.3-70b-versatile | $0.59 | $0.79 |

### LLMClient Changes

After each API call, extract usage from the response and publish a `usage.recorded` event:

```typescript
bus.publish('usage.recorded', agentId, {
  taskId,
  agentId,
  model,
  inputTokens: usage.input_tokens,
  outputTokens: usage.output_tokens,
  cacheReadTokens: usage.cache_read_tokens ?? 0,
  cacheWriteTokens: usage.cache_write_tokens ?? 0,
});
```

A `UsageTracker` service subscribes to this event and writes to the DB.

### WebSocket Handlers

```
'usage.summary'        → { period: 'day'|'week'|'month', agentId?: string }
'usage.by_agent'       → top agents by token usage for period
'usage.by_task'        → token breakdown for a specific task
'usage.totals'         → all-time totals
```

### Dashboard

1. **Agent cards** (AgentsPage) — add small token count badge: `12.4k tokens today`
2. **Task detail** (Tasks tab from Spec 005) — show token breakdown for each task
3. **Usage page** — new dashboard page:
   - Bar chart: daily token usage by model (last 30 days)
   - Agent leaderboard: top 5 agents by cost this week
   - Total cost this month (estimated)

---

## Test Criteria

- LLMClient publishes usage.recorded after each API call
- UsageTracker writes to DB on event
- usage.summary returns correct aggregates for day/week/month
- Model pricing used correctly in cost estimation
- No regression on existing tests

---

## Rollback

UsageTracker is a passive subscriber — if disabled, all other functionality is unaffected. Feature flag: `FORT_USAGE_TRACKING=false`.
