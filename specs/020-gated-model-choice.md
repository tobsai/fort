---
id: 020-gated-model-choice
title: Choice gate when a model is gated/rate-limited
status: draft
---

## Goal

When an agent's LLM call is rejected because the model is gated (HTTP 429 — e.g. a Claude subscription rejecting opus/sonnet while haiku still flows), give the user an explicit choice instead of silently degrading: **switch to another configured provider**, **drop to a lighter model on the same provider**, or **use an API key instead of the subscription**. The choice is presented in the portal chat as a blocking card and can be remembered per-agent.

Background, scheduled, subtask, and CLI work — where no human is watching — keep the existing automatic tier fallback (spec shipped in v0.3.9) so unattended runs never stall.

## Why now

A real incident: at 24% weekly / 36% 5-hour utilization (both "allowed"), Anthropic returned `429 rate_limit_error` for `claude-opus-4-6` and `claude-sonnet-4-5` while `claude-haiku-4-5` returned 200. The haiku response carried `anthropic-ratelimit-unified-overage-status: rejected` / `overage-disabled-reason: out_of_credits` — i.e. premium models are hard-gated once the premium lane is consumed and pay-as-you-go overage is unavailable, independent of the broad plan counter the user sees.

v0.3.9 made the tool loop auto-fall-back (opus→sonnet→haiku) so the agent at least works. But silent downgrade is the wrong default when a human is present: the user has a second, un-gated subscription (ChatGPT/Codex) and may prefer to route premium work there, or to spend on the Anthropic API, rather than quietly get haiku-quality answers. The multi-provider routing from spec-era work (`SpecialistIdentity.provider`, per-agent runtime resolution) makes the alternatives real and selectable.

## Approach

### Trigger and scope

- A task is **interactive** iff `task.source === 'user_chat'`. The Specialist passes `interactive: true` on the `LLMRequest` for those tasks; everything else (`background`, subtasks with `parentId`, CLI one-shots) is non-interactive.
- **Non-interactive:** unchanged — the v0.3.9 auto tier-fallback in `callWithRetry` / the tool loops stays. If a remembered choice exists on the identity, it already applies via per-agent routing.
- **Interactive:** when the LLM client would otherwise auto-fall-back from a gated 429, it instead throws `ModelGatedError` carrying the gated model/tier, the current provider, and the viable alternatives. The Specialist catches it and runs the **choice gate**.

### ModelGatedError (new)

`packages/core/src/llm/index.ts`:

```ts
export class ModelGatedError extends Error {
  constructor(
    public gatedModel: string,
    public gatedTier: ModelTier,
    public providerId: string,            // current provider, e.g. 'anthropic'
    public viableProviders: Array<{ id: string; name: string; powerfulModel: string }>,
    public viableTiers: ModelTier[],      // lower tiers on the SAME provider not in cooldown
    public canUseApiKey: boolean,         // true if currently on subscription auth for this provider
  ) { super('__MODEL_GATED__'); this.name = 'ModelGatedError'; }
}
```

- `callWithRetry` keeps throwing the internal `__TIER_FALLBACK__` signal on a gated 429 as today. The **decision** to convert it into a user choice happens at the existing catch sites that currently auto-switch: the plain `complete()` catch (~line 725) and the two tool-loop catches (~line 894 Anthropic, ~line 2567 OpenAI, both added in v0.3.9). When `request.interactive` is set, those sites throw `ModelGatedError` instead of recursing/switching; otherwise they auto-switch exactly as today.
- `ModelGatedError` is built from data already on hand at those sites: `getNextAvailableTier` for `viableTiers`, `getAvailableProviders()` for `viableProviders` (filtered `usable && id !== providerId`), and `this._isOAuthToken`/`authMethod` for `canUseApiKey`.

### Choice gate (new) — mirror the tool-approval pattern

Model the blocking flow on `ToolExecutor.awaitApproval` (`packages/core/src/tools/executor.ts:243-288`): a resolver Map + bus event + 10-minute timeout.

New `packages/core/src/services/model-choice.ts` — `ModelChoiceService`:
- `requestChoice(req): Promise<ModelChoice>` — stores a resolver in a `pendingChoices` Map keyed by a generated id, publishes `model-choice.required` with `{ id, taskId, agentId, gatedModel, options }`, returns the promise. Times out at 10 min → resolves to `{ action: 'fallback' }` (degrade to lowest viable tier, matching non-interactive behavior).
- `resolveChoice(id, choice)` — resolves the pending promise. Called by the WS handler.
- `options` are built from `ModelGatedError`: one `switch_provider` per viable provider, one `lighter_model` (lowest viable tier), `use_api_key` if `canUseApiKey`. Each carries a `remember: boolean` from the user.

Wire it into `Fort` (sibling of `hatch`), and inject into the Specialist so `execute()` can call it.

### Specialist integration

`packages/core/src/agents/specialist.ts` (around the LLM call at lines 270-307):
- Set `interactive: task.source === 'user_chat'` on the `complete`/`completeWithTools` request.
- Wrap the call in a retry loop (max ~4 rounds to bound re-prompting):
  1. Call the LLM.
  2. On `ModelGatedError`: set task status `blocked`; `await modelChoice.requestChoice(...)`; apply the result:
     - `switch_provider` → use that provider for the retry; if `remember`, set `identity.provider` and persist `identity.yaml`.
     - `lighter_model` → use that tier for the retry; if `remember`, set `identity.defaultModelTier` and persist.
     - `use_api_key` → write the key via `LLMClient.writeEnvFileValue` / provider store, force `refreshAuth()`, retry on API-key auth; if `remember`, persist an auth preference (see Open question below).
     - `fallback` (timeout) → lowest viable tier, no persistence.
  3. Set task back to `in_progress` and retry. If the chosen route is *also* gated, the loop re-throws `ModelGatedError` with the now-smaller option set (the just-tried tier/provider removed via cooldown) → card re-shows without it. After max rounds, degrade to fallback and proceed.
- Unremembered choices are **per-task**: the next `user_chat` task starts fresh and re-prompts if gated (no session memory in v1).
- **Mid-loop limitation (v1):** because `ModelGatedError` unwinds the whole `completeWithTools` call, the Specialist retry re-invokes the tool loop from the top. If gating strikes *after* tools already ran this turn, those tools re-execute. In practice gating almost always hits the first call (e.g. the greeting / first message before any tool runs), so this is accepted for v1; a mid-loop inline switch that preserves loop state is a follow-up.

### Server (bus ↔ WS bridge)

`packages/core/src/server/index.ts`:
- Subscribe to `model-choice.required` → `broadcast({ type: 'model-choice.new', payload })` (next to the `approval.required` bridge at line 277).
- New WS message `case 'model-choice.respond'`: `{ id, action, providerId?, tier?, apiKey?, remember }` → `fort.modelChoice.resolveChoice(id, ...)` (mirrors `approval.respond` at line ~1288).
- The existing `chat` handler already `await`s the full task, so the chat request stays open until the user answers and the task completes.

### Dashboard

`packages/dashboard/src/pages/ChatPage.tsx`:
- New inline message role `model-choice` (reuse the `classification`/`plan` inline-card rendering already in the message list).
- Subscribe to `model-choice.new` → push the card into the selected agent's stream with a "thinking/paused" affordance.
- Card renders the dynamic options + a "Remember for this agent" checkbox; on click sends `model-choice.respond`. Once answered, the card collapses to a one-line summary ("Switched to OpenAI") and the agent's real reply streams in via the existing `chat.response` path.
- `use_api_key` with no key configured expands an inline password field (reuse `POST /api/providers/setup`) before sending the response.

### Task status

Use the existing unused `blocked` status (`packages/core/src/types.ts` TaskStatus) while awaiting the choice; restore to `in_progress` on resume. This makes a paused task visible on the board.

## Affected files

- `packages/core/src/llm/index.ts` — `ModelGatedError`; throw it on gated 429 when `request.interactive`; add `interactive?: boolean` to `LLMRequest`.
- `packages/core/src/services/model-choice.ts` — new `ModelChoiceService` (resolver map, bus event, timeout).
- `packages/core/src/fort.ts` — construct + wire `ModelChoiceService`; inject into Specialist/AgentFactory.
- `packages/core/src/agents/specialist.ts` — set `interactive`, catch `ModelGatedError`, run the gate, apply+persist choice, retry loop.
- `packages/core/src/server/index.ts` — `model-choice.required`→WS bridge; `model-choice.respond` handler.
- `packages/dashboard/src/pages/ChatPage.tsx` — choice card + round-trip.
- `packages/dashboard/src/types/index.ts` — `model-choice` message type.
- `packages/dashboard/src/utils/api.ts` — none expected (uses WS), unless inline key paste reuses the REST helper.

## Test criteria

- **Unit (`packages/core/src/__tests__/`):**
  - LLM client throws `ModelGatedError` (not `__TIER_FALLBACK__`) on a gated 429 when `interactive: true`, with correct `viableProviders`/`viableTiers`/`canUseApiKey`; still auto-falls-back when `interactive: false`. Mirror the existing rate-limit mocking in `llm.test.ts`.
  - `ModelChoiceService.requestChoice` resolves when `resolveChoice` is called; times out to `{ action: 'fallback' }`.
  - Specialist applies a `switch_provider` choice and, with `remember`, persists `identity.provider` (assert via `agentFactory.getIdentity`).
- **Integration:** simulate a `user_chat` task whose first LLM call is gated → assert task goes `blocked`, a `model-choice.required` event fires, resolving it resumes the task to `completed`.
- **Manual (portal):** with opus/sonnet gated and OpenAI configured, send a chat → card appears with "Switch to OpenAI / lighter model / use API key" → pick OpenAI → reply streams from gpt-5.x; with "remember" checked, `~/.fort/agents/<id>/identity.yaml` shows `provider: openai`.
- **Regression:** background/greeting-from-non-chat paths still auto-fall-back (no card, no stall).

## Rollback

Self-contained and additive. To roll back: stop throwing `ModelGatedError` (revert the `interactive` branch in the LLM client) — the Specialist's catch becomes dead and the auto-fallback path resumes for all tasks. The new service, bus events, and dashboard card are inert without the trigger. No schema/migration changes (uses existing `blocked` status; no new tables).

## Open questions (resolved during brainstorming)

- Interaction model: **blocking in-chat card** (not silent fallback, not setup-time policy).
- Remember scope: **permanent**, persisted to `identity.yaml`.
- Unremembered scope: **per-task** — re-prompt next gated message.
- Non-interactive: **auto-fallback** (keep v0.3.9).
- Lighter-also-gated: **re-show card without that option**.

### Still to decide in planning
- `use_api_key` "remember": where the auth preference lives. Options: a new `identity.authPreference?: 'subscription' | 'api_key'` field, or simply that writing an API key to `~/.fort/.env` + the provider store is itself the durable signal (runtime already prefers an explicit key). Lean toward the latter (no new field) unless planning surfaces a conflict.
