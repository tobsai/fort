---
sidebar_position: 8
title: Tokens & Budgets
---

# Tokens & Budgets

Fort tracks every token consumed across all LLM calls and provides budget controls to prevent runaway costs. Token data is stored in SQLite alongside the rest of Fort's operational data.

## Automatic Tracking

Every LLM call is logged automatically with:

- **Model** — Which tier and model was used
- **Input tokens** — Tokens in the prompt (including injected behaviors and memory)
- **Output tokens** — Tokens in the response
- **Estimated cost** — Calculated from per-model pricing
- **Task ID** — Which task triggered the call
- **Agent** — Which agent made the call
- **Timestamp** — When the call was made

No configuration is needed. Tracking is always on.

## Viewing Stats

Get a summary of token usage:

```bash
fort tokens stats
```

```
Period: 2026-03-21 (today)
Total Calls: 47
Total Tokens: 142,300 (in: 98,200 / out: 44,100)
Estimated Cost: $1.24

Period: 2026-03 (this month)
Total Calls: 892
Total Tokens: 3,241,000 (in: 2,180,000 / out: 1,061,000)
Estimated Cost: $28.17
```

### Breakdown by Agent

See which agents consume the most tokens:

```bash
fort tokens stats --by-agent
```

```
AGENT              CALLS   TOKENS      COST
Orchestrator       312     1,240,000   $10.80
Email Triager      201     890,000     $7.44
Code Reviewer      156     720,000     $6.30
Reflection         89      210,000     $2.10
Memory             134     181,000     $1.53
```

### Breakdown by Model

See usage across model tiers:

```bash
fort tokens stats --by-model
```

```
MODEL          CALLS   TOKENS      COST
haiku-4.5      445     1,020,000   $3.06
sonnet-4.6     312     1,580,000   $12.80
opus-4.6       135     641,000     $12.31
```

### Time Range

Filter stats to a specific period:

```bash
fort tokens stats --from 2026-03-01 --to 2026-03-15
```

## Setting Budgets

Set daily and monthly token limits to prevent unexpected spend:

```bash
fort tokens budget-set \
  --daily 500000 \
  --monthly 10000000 \
  --warning 0.8
```

- `--daily` — Maximum tokens per day
- `--monthly` — Maximum tokens per calendar month
- `--warning` — Threshold (0.0-1.0) at which a warning fires; `0.8` means warn at 80% of budget

### Budget Events

Fort publishes events on the ModuleBus when budgets are approached or exceeded:

| Event | Trigger |
|---|---|
| `tokens:warning` | Usage crosses the warning threshold |
| `tokens:exceeded` | Usage exceeds the budget limit |

When `tokens:exceeded` fires, Fort blocks further LLM calls for that period. Existing in-progress calls are allowed to finish, but new calls return an error until the budget resets.

```typescript
bus.on('tokens:warning', (event) => {
  console.log(`Token warning: ${event.data.usage}/${event.data.limit} (${event.data.period})`);
});

bus.on('tokens:exceeded', (event) => {
  console.log(`Budget exceeded for ${event.data.period}. LLM calls blocked.`);
});
```

## Viewing Current Budget

Check budget status:

```bash
fort tokens budget
```

```
Daily Budget:   500,000 tokens
Daily Usage:    142,300 tokens (28.5%)
Daily Warning:  at 400,000 tokens (80%)

Monthly Budget: 10,000,000 tokens
Monthly Usage:  3,241,000 tokens (32.4%)
Monthly Warning: at 8,000,000 tokens (80%)

Status: OK
```

## Per-Task Tracking

Every task in the TaskGraph records its own token usage. Inspect a task to see how many tokens it consumed:

```bash
fort tasks inspect tk-201
```

```
Task: tk-201
Routine: Morning Briefing
Status: success
Duration: 12s
Token Usage:
  Calls: 3
  Input: 3,100 tokens
  Output: 1,130 tokens
  Cost: $0.04
  Models: haiku-4.5 (2), sonnet-4.6 (1)
```

This makes it easy to identify expensive tasks and optimize them — for example, switching a routine from `standard` to `fast` tier if the quality is acceptable.

## Configuration

Budget settings are stored in `.fort/config.yaml`:

```yaml
tokens:
  daily_limit: 500000
  monthly_limit: 10000000
  warning_threshold: 0.8
  pricing:
    haiku-4.5:
      input_per_1k: 0.001
      output_per_1k: 0.005
    sonnet-4.6:
      input_per_1k: 0.003
      output_per_1k: 0.015
    opus-4.6:
      input_per_1k: 0.015
      output_per_1k: 0.075
```

Update pricing if Anthropic changes rates. Fort does not auto-update pricing.
