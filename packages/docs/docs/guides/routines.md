---
sidebar_position: 6
title: Routines
---

# Routines

Routines are scheduled tasks that run automatically on a cron schedule. They are the backbone of Fort's proactive behavior — morning summaries, inbox triage, end-of-day reviews, and periodic maintenance all run as routines.

## Adding a Routine

Create a routine with a cron schedule:

```bash
fort routines add \
  --name "Morning Briefing" \
  --schedule "0 9 * * 1-5" \
  --description "Summarize overnight emails, calendar, and pending tasks"
```

- `--name` — Human-readable name
- `--schedule` — Standard 5-field cron expression
- `--description` — What the routine does (passed to the executing agent as context)

Common cron patterns:

| Expression | Meaning |
|---|---|
| `0 9 * * 1-5` | Weekdays at 9:00 AM |
| `0 17 * * *` | Every day at 5:00 PM |
| `0 */2 * * *` | Every 2 hours |
| `0 9 1 * *` | First of every month at 9:00 AM |
| `30 8 * * 1` | Mondays at 8:30 AM |

## Listing Routines

View all routines and their next scheduled run:

```bash
fort routines list
```

```
ID    NAME               SCHEDULE       STATUS    NEXT RUN
r-01  Morning Briefing   0 9 * * 1-5    enabled   2026-03-24 09:00
r-02  Inbox Triage       0 */2 * * *    enabled   2026-03-21 14:00
r-03  Weekly Reflection  0 17 * * 5     enabled   2026-03-21 17:00
r-04  Dependency Audit   0 9 1 * *      disabled  2026-04-01 09:00
```

## Manual Trigger

Run a routine immediately without waiting for its schedule:

```bash
fort routines run r-01
```

```
Routine r-01 (Morning Briefing) triggered manually.
Task created: tk-201
```

This creates a task in the TaskGraph identical to a scheduled execution.

## Enable and Disable

Disable a routine to stop it from running on schedule:

```bash
fort routines disable r-04
```

Re-enable it later:

```bash
fort routines enable r-04
```

Disabled routines keep their schedule and history but do not trigger automatically. They can still be run manually.

## Execution History

View past executions of a routine:

```bash
fort routines history r-01
```

```
RUN    TASK      STATUS     STARTED              DURATION   TOKENS
#47    tk-201    success    2026-03-21 09:00:02   12s       4,230
#46    tk-198    success    2026-03-20 09:00:01   15s       5,102
#45    tk-195    success    2026-03-19 09:00:03   11s       3,890
#44    tk-190    failed     2026-03-18 09:00:01   3s        120
```

Drill into a specific execution by inspecting its task:

```bash
fort tasks inspect tk-201
```

## TaskGraph Integration

Every routine execution creates a task in the TaskGraph. The task captures:

- The routine's description (used as the task prompt)
- The agent that executed it
- All LLM calls, tool invocations, and memory accesses
- Token usage and cost
- Success or failure status with error details

This means routine executions have the same traceability as any interactive thread.

## Attaching Behaviors to Routines

Routines can have behaviors that only apply during their execution. This lets you fine-tune how an agent behaves for a specific scheduled task:

```bash
fort routines add \
  --name "EOD Summary" \
  --schedule "0 17 * * 1-5" \
  --description "Summarize what was accomplished today" \
  --behaviors "Keep summary under 200 words,Highlight blocked items"
```

These behaviors are injected into the LLM prompt only when this routine runs. They do not affect other tasks handled by the same agent.

## Editing Routines

Update a routine's schedule or description:

```bash
fort routines edit r-01 --schedule "0 8 * * 1-5"
fort routines edit r-01 --description "Summarize emails and calendar for the day"
```

## Deleting Routines

Remove a routine permanently:

```bash
fort routines delete r-04
```

This removes the routine definition but preserves its execution history in the TaskGraph.
