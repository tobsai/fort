---
id: 018-agent-voice-and-hatch
title: Agent voice, hatch, goals, and reflection
status: approved
---

## Goal
Replace Fort's mechanical, action-oriented default voice with a mode-adaptive one that gives terse direct answers for lookups but leans into curious collaboration for substantive work. Add a conversational "hatch" — a free-form first-contact session after `fort init` where the agent gets to know the user and proposes goals back. Make goals first-class structured objects that the agent uses as context across every chat and that the existing `ReflectionService` periodically reviews to nudge stale goals or draft next-step tasks for user approval.

## Approach

### Voice & mode detection (`packages/core/src/llm/index.ts`)
- Rewrite the base system prompt (line 233) to define **quick mode** (terse direct answer) and **curious mode** (propose options, one clarifying question at a time, personally addressed).
- The model picks per-message; no regex classifier.
- Hard constraints: override authority (lean in when a direct answer would mislead), no inner monologue, never stack questions.
- `buildSystemPrompt` (line 2047) is extended to inject active goals (title + status) and top-N profile facts on every user-facing request, alongside the existing soul/behaviors/memory/time sections.

### Hatch — conversational onboarding (`packages/core/src/services/hatch/`)
- New service. Adds `hatchedAt?: string | null` to `SpecialistIdentity`. New agents start with `hatchedAt: null`.
- When the portal loads an agent thread with `hatchedAt: null`, the server posts an agent-initiated first message and runs a hatch loop: each user reply triggers an LLM call with the hatch system prompt (soft agenda: who you are, what you're working on, top things you're moving, working style, hard rules — one thing at a time, follow user's lead).
- Profile facts surfaced during the chat are written to the existing memory graph with a `profile` tag, agent-partitioned.
- When the model signals it has enough (structured field in response), the service proposes goals back as a chat message. User confirms/edits in chat. On confirmation: goals persisted, `hatchedAt` set.
- Mid-hatch disconnect: resume from memory; no restart. User refusal: set `hatchedAt: <now>` with no goals; reflection won't run until goals exist.
- `fort init` no longer auto-launches anything; it prints *"Run `fort portal` to meet [agent name]."* Portal-first users still get `SetupWizard.tsx`, which converges on the same hatch.

### Goals (`packages/core/src/services/goals/`)
- New SQLite table `goals(id, agent_id, title, description, status, source, created_at, updated_at, last_activity_at)`. `status` ∈ {active, paused, achieved, abandoned}. `source` ∈ {hatch, user, agent_proposed}.
- New `goal_id TEXT NULL` column on `tasks` (schema migration in `task-store.ts`).
- After decomposition, `task-planner.ts:decomposeTask` runs a cheap LLM call against active goals to tag each subtask. Low confidence → `goal_id: null`.
- New REST endpoints `GET/POST/PATCH /api/goals`; `GET /api/tasks?goalId=…`.
- New CLI: `fort goals list/add/edit/done`.
- Drop `## Goals` prose section from `packages/cli/assets/triager/SOUL.md` and from the portal SOUL generator at `server/index.ts:1576`; replace with marker comment. Structured store is the source of truth.

### Reflection (`packages/core/src/services/reflection/`)
- Extend existing service with a goal pass on a 24h schedule (configurable).
- Score each active goal: **staleness** (days since `last_activity_at`, default flag at 7), **blockers** (tasks `failed` or `created`>3d).
- For each flagged goal, one standard-tier LLM call → `{action: 'nudge'|'draft_task'|'skip', message?, task?}`.
- Apply: nudge posts a chat message; draft_task creates a `status: 'draft'` task surfaced in chat for user approval (Tier 3 Permission gate stays in place).
- Bounded: never executes externally without user approval. Rate-limited to one nudge per goal per 48h; cooldown doubles when ignored.
- `fort reflection on/off/status` for control. Default on after hatch completes.

## Affected Files
- `specs/018-agent-voice-and-hatch.md` (this file)
- `packages/core/src/llm/index.ts` (base prompt line 233; `buildSystemPrompt` line 2047)
- `packages/core/src/types.ts` (add `Goal`, `hatchedAt`, `goal_id`)
- `packages/core/src/services/hatch/index.ts` (new)
- `packages/core/src/services/hatch/prompt.ts` (new)
- `packages/core/src/services/goals/index.ts` (new)
- `packages/core/src/services/goals/store.ts` (new)
- `packages/core/src/services/reflection/index.ts` (extend)
- `packages/core/src/services/reflection/scoring.ts` (new)
- `packages/core/src/task-graph/task-store.ts` (schema migration)
- `packages/core/src/agents/task-planner.ts` (goal-tagging in `decomposeTask`)
- `packages/core/src/server/index.ts` (hatchedAt on create; SOUL template; new endpoints; hatch loop integration)
- `packages/cli/src/commands/init.ts` (print "run fort portal")
- `packages/cli/src/commands/goals.ts` (new)
- `packages/cli/src/commands/reflection.ts` (new)
- `packages/cli/assets/triager/SOUL.md` (drop `## Goals` section)
- `packages/dashboard/src/components/SetupWizard.tsx` (trigger hatch on completion)
- `packages/dashboard/src/` (Goals lens on kanban — minimal)
- `packages/core/src/__tests__/llm-prompt.test.ts` (snapshot, new)
- `packages/core/src/__tests__/hatch.test.ts` (new)
- `packages/core/src/__tests__/goals.test.ts` (new)
- `packages/core/src/__tests__/reflection.test.ts` (new)

## Test Criteria
- `buildSystemPrompt` snapshot tests cover: with/without soul, with/without goals, with/without profile facts. Each variant stable across runs.
- Quick prompt ("what time is it?") yields a terse response; substantive prompt ("should I use SQLite or Postgres for X?") yields one clarifying question.
- An ambiguous short prompt ("should I ship it?") triggers override authority — agent asks for context, doesn't guess.
- `fort init` from clean state prints "Run `fort portal` to meet [agent name]" and does not auto-launch.
- Opening the portal with `hatchedAt: null` triggers an agent-initiated first message; the conversation captures profile facts (visible in memory) and ends with a goal proposal; user confirmation sets `hatchedAt` and persists goals.
- Mid-hatch disconnect → next portal open resumes without restart.
- `fort goals list` shows hatch-confirmed goals. Goal CRUD via CLI and API both work.
- A substantive chat-to-task message produces a decomposed task whose subtasks carry `goal_id` for matching goals.
- A fresh thread after hatch demonstrates cross-chat continuity: agent uses goal + profile context without being told.
- Reflection: a goal manually aged past staleness threshold produces a nudge or drafted task in the goal's agent thread on next pass. A second pass within the cooldown window produces no second nudge. Ignored nudges double the cooldown.
- `fort reflection off` halts the loop; `fort reflection status` reports state.
- All 326 existing tests still green; new suites pass.

## Rollback Plan
- Voice: revert `packages/core/src/llm/index.ts` to the prior base system prompt; redeploy.
- Hatch: set `hatchedAt` to a non-null value for any agent stuck mid-hatch (`fort goals` or direct YAML edit), then revert hatch service and portal first-message logic; hatch loop becomes inert.
- Goals: `DROP TABLE goals;` and remove the `goal_id` column from `tasks`. The structured-goals injection in `buildSystemPrompt` no-ops when the table is missing (defensive check). Drop the goals CLI commands.
- Reflection: `fort reflection off` halts the goal pass without touching the rest of the service; revert `reflection/index.ts` to restore the prior periodic-review behavior.
- Each section is committed independently so any subset can be reverted via `git revert <sha>`.
