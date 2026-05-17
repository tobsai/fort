---
id: 019-skills-with-tools
title: Skills with Tools at the core
status: draft
---

## Goal
Make Fort accumulate capability instead of re-deriving it. Every time the agent does something useful via ad-hoc reasoning, that procedure should become a candidate **Skill** — a named, parameterized, testable workflow that composes Fort-owned **Tools** plus deterministic LLM steps. The next time the same pattern appears, Fort runs the Skill instead of re-thinking the problem from scratch. Skills can request Tools that don't exist yet, triggering tool-creation as a subtask. The Reflection service distills repeated ad-hoc work into Skill proposals.

This generalizes the existing Triager pattern (a single hand-built classifier) into a first-class abstraction: any narrow, repeatable capability — drafting a particular kind of email, running a specific report, classifying a category of message, summarizing a meeting in a particular shape — becomes a Skill the user can edit, version, disable, or hand to another agent.

## Why now
- Fort's current task pipeline is reflexive: every chat → classify → decompose → execute as a fresh plan. Repeated patterns burn LLM tokens re-discovering the same workflow each time.
- `CLAUDE.md` already names the principle ("Fort uses tools that are powered by AI but deterministic at runtime") and the hierarchy (check ToolRegistry → wrap industry tool → build new). Skills are the missing piece *above* tools: a tool answers *can I do this step*; a skill answers *how do I do this whole job*.
- Without an accumulation layer, "self-improving" remains aspirational. Skills make the self-improvement loop concrete and visible (`fort skills list`, `fort skills run <name>`, `fort skills propose`).

## Approach

### Data model
- **Skill** (new SQLite table `skills`):
  - `id`, `name` (slug), `display_name`, `description`
  - `version` (semver-ish int that increments on edit)
  - `input_schema` (JSON Schema) — typed parameters the skill needs
  - `output_schema` (JSON Schema) — what it produces
  - `steps` (JSON array of `SkillStep`s — see below)
  - `tool_deps` (array of tool IDs the skill expects in the registry)
  - `agent_id` (owner — skills live per-agent for partitioning + voice; can be promoted to `global` later)
  - `success_criteria` (free text + optional structured assertions)
  - `created_at`, `updated_at`, `created_by` ('user' | 'agent_proposed' | 'distilled')
  - `status` ('active' | 'draft' | 'archived')
  - `last_run_at`, `run_count`, `success_count` (lifecycle stats)
- **SkillStep** (one of):
  - `{ kind: 'tool_call', toolId, paramTemplate }` — invokes a Fort Tool with templated params
  - `{ kind: 'llm', model, promptTemplate, expectedShape? }` — deterministic LLM step with a typed output expectation
  - `{ kind: 'branch', condition, then: SkillStep[], else: SkillStep[] }` — small conditional
  - `{ kind: 'parallel', steps: SkillStep[] }` — fan-out, gather
  - Intentionally minimal — anything more complex composes child skills.
- **SkillRun** (new table `skill_runs`):
  - `id`, `skill_id`, `skill_version`, `task_id`, `agent_id`
  - `input` (JSON), `output` (JSON), `status` ('running' | 'succeeded' | 'failed')
  - `step_log` (JSON array of per-step traces — tool call params/results, LLM input/output)
  - `started_at`, `finished_at`, `duration_ms`, `cost_usd`
  - Powers replay, regression testing, and the distillation loop.

### Services (new files)
- `packages/core/src/services/skills.ts` — `SkillsService`: CRUD on skills, dispatch (`run(name, input, context)`), lifecycle stats.
- `packages/core/src/services/skills-store.ts` — SQLite store + migrations.
- `packages/core/src/services/skill-runner.ts` — executes a `SkillStep[]` against a context. Pure orchestration; delegates tool calls to `ToolExecutor` and LLM steps to `LLMClient`. Emits step-level events to the bus.
- `packages/core/src/services/skill-distiller.ts` — Reflection-driven proposal of skills from repeated ad-hoc task patterns. Pure-function scoring (count similar tasks within a window) + one LLM call per cluster to draft a skill spec.
- `packages/core/src/services/skill-validator.ts` — runs a candidate skill against a small fixture set before it can transition `draft` → `active`. Borrows the existing `Harness` pattern.

### Wiring
- Add `SkillsService` and `SkillRunner` to `Fort` (sibling of `goals`, `hatch`, `reflection`).
- Extend `LLMRequest` with `injectSkills?: boolean` (default true on user-facing chats) — `buildSystemPrompt` lists the names + 1-line descriptions of active skills for the current agent under `## Available Skills`. The LLM picks one when relevant; otherwise it plans ad-hoc.
- Extend the chat-to-task pipeline (`agents/task-planner.ts`): after the Triager classifies a chat as a task, before decomposition, ask "does an existing skill match this?" via a cheap LLM call. If yes, dispatch `skills.run(name, input)` instead of decomposing. If no, decompose normally and tag the resulting task with `metadata.distillationCandidate = true` so Reflection can later propose a skill.
- Extend `ReflectionService` with a `reviewSkillCandidates()` pass on the 24h schedule: clusters recent tasks tagged as candidates, proposes skill drafts via `skill-distiller`, and posts them in chat for the user to review/approve.

### CLI + API
- `fort skills list/show/run/edit/disable/archive`
- `fort skills propose` — manual trigger for the distillation pass.
- REST endpoints under `/api/skills` (CRUD + run + run-history).
- Portal "Skills" lens — list view, last-run stats, JSON editor for steps, "Run with input…" form.

### Skill-builds-tool
When a `tool_call` step references a tool ID that isn't registered, the runner does NOT silently fail. It pauses the run, creates a `Build tool: <name>` task (reusing the existing `extractToolProposal` flow in `specialist.ts`), surfaces an approval to the user, and resumes the skill run once the tool is registered. This is the literal "skills build tools" feedback loop the directive calls for.

### Distillation loop (the self-improvement bit)
1. Every completed task with `metadata.distillationCandidate` is recorded for clustering.
2. On the reflection schedule, group candidates by structural similarity (LLM clustering pass, fast tier).
3. For each cluster with N≥3 occurrences in the last 30 days, the distiller proposes a skill spec (name, input schema, steps).
4. The proposal is posted to the relevant agent's chat thread: *"I've done this kind of thing 5 times — want me to make it a skill? [proposal]"*
5. On user approval, the skill is created in `draft` status. `skill-validator` runs it against the historical cases as fixtures; if ≥80% pass, it's promoted to `active`. Otherwise the skill stays draft and the user can edit.

### Safety
- Skills with tool calls that have approval tiers (Tier 3+) propagate those approvals — the runner never bypasses the user's permission model.
- A skill's first 3 runs after creation always require user approval, regardless of tool tier. This is the "trust on display" period.
- Versioning: editing an active skill creates a new version; previous version remains queryable via `SkillRun.skill_version` for postmortem.

## Affected Files
- `specs/019-skills-with-tools.md` (this file)
- `packages/core/src/services/skills.ts` (new)
- `packages/core/src/services/skills-store.ts` (new)
- `packages/core/src/services/skill-runner.ts` (new)
- `packages/core/src/services/skill-distiller.ts` (new)
- `packages/core/src/services/skill-validator.ts` (new)
- `packages/core/src/types.ts` — `Skill`, `SkillStep`, `SkillRun` interfaces
- `packages/core/src/fort.ts` — wire `skills`, `skillRunner`
- `packages/core/src/llm/index.ts` — `injectSkills` in `buildSystemPrompt`
- `packages/core/src/agents/task-planner.ts` — skill-match step before decompose
- `packages/core/src/services/reflection.ts` — `reviewSkillCandidates()`
- `packages/core/src/server/index.ts` — `/api/skills` endpoints
- `packages/cli/src/commands/skills.ts` (new)
- `packages/cli/src/index.ts` — register skills command
- `packages/dashboard/src/` — Skills lens
- Tests for store, runner, distiller, validator, end-to-end via Fort

## Test Criteria
- A skill with a single `tool_call` step runs end-to-end and writes a `SkillRun` row with the tool's output.
- A skill with a `tool_call` referencing an unregistered tool ID pauses the run, creates a `Build tool:` task, and resumes after the tool is registered.
- Inserting an active skill changes `buildSystemPrompt` output to include it under `## Available Skills`.
- A chat-to-task message matching an active skill's input schema dispatches `skills.run` instead of `decomposeTask`.
- A cluster of 3+ similar candidate tasks within 30 days produces a skill proposal posted to the relevant agent's chat.
- A draft skill that passes ≥80% of fixture runs auto-promotes to active.
- First 3 runs of any newly-active skill require user approval regardless of tool tiers.
- All existing tests continue to pass.

## Rollback Plan
- Set the `SkillsService` flag `enabled: false` in config — chat-to-task pipeline reverts to the existing classify → decompose flow without skill matching.
- Drop tables: `DROP TABLE skill_runs; DROP TABLE skills;`. Skill-distillation candidate tagging on tasks (a metadata flag) is harmless if left in place.
- Revert `buildSystemPrompt` injection of `## Available Skills` — system prompt assembly is monotonic, removing the section has no side effects.
- The "Build tool" pause-and-resume is a new code path; if it misbehaves, set `skill_runner.toolMissingPolicy = 'fail'` to revert to old behavior of failing the run.

## Open Questions
1. Should skills be agent-scoped or global by default? (Spec proposes agent-scoped with explicit promotion; alternative is global with agent filters.)
2. Storage for `steps` — JSON in SQLite is fine for small skills but limits queryability. Acceptable for v1; reconsider if step counts grow.
3. Should the distiller propose skills only after user-confirmed task patterns, or also from agent_delegation? (Spec proposes both, gated by candidate tagging.)
4. Versioning semantics: do edits to an active skill auto-bump version, or require an explicit `fort skills publish`? (Spec leans auto-bump; explicit publish is more disciplined but adds friction.)
