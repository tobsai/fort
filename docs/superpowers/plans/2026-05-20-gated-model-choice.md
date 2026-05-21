# Gated-Model Choice Gate — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When an interactive portal chat hits a gated/rate-limited model (HTTP 429), pause and let the user choose — switch provider, drop to a lighter model, or use an API key — instead of silently degrading. Background work keeps the existing auto-fallback.

**Architecture:** The LLM client throws a new `ModelGatedError` (instead of auto-switching) when a request is marked `interactive` and a gated 429 occurs. The Specialist catches it, blocks on a new `ModelChoiceService` (a resolver-map + bus-event + timeout, modeled exactly on `ToolExecutor.awaitApproval`), and retries the call with the chosen routing — persisting the choice to the agent's `identity.yaml` if "remember" was checked. The portal renders an inline choice card and answers over the existing WS round-trip.

**Tech Stack:** TypeScript (core + dashboard React), Vitest, better-sqlite3 (no new tables), ModuleBus events, WebSocket.

**Reference spec:** `specs/020-gated-model-choice.md`

---

## File structure

- `packages/core/src/llm/index.ts` — add `interactive?` + `providerOverride?` to `LLMRequest`; `ModelGatedError` class; honor `providerOverride` in `resolveRuntimeProvider`; throw `ModelGatedError` at the 3 existing tier-fallback catch sites when `request.interactive`.
- `packages/core/src/services/model-choice.ts` — **new** `ModelChoiceService` (resolver map, bus event, 10-min timeout, identity persistence via `AgentFactory.updateIdentity`).
- `packages/core/src/fort.ts` — construct + wire `ModelChoiceService`; inject into `AgentFactory`.
- `packages/core/src/agents/hatchery.ts` — `setModelChoice()` on the factory; pass to specialists alongside `setLLM`.
- `packages/core/src/agents/specialist.ts` — `setModelChoice()`; set `interactive` on `user_chat` LLM requests; catch `ModelGatedError`, run the gate, apply choice + retry loop.
- `packages/core/src/server/index.ts` — `model-choice.required` → WS `model-choice.new` bridge; `model-choice.respond` WS handler.
- `packages/dashboard/src/types/index.ts` — add `"model-choice"` to `ChatMessage.role`; add the payload shape.
- `packages/dashboard/src/pages/ChatPage.tsx` — subscribe to `model-choice.new`, render the card, send `model-choice.respond`.

---

## Task 1: LLMRequest gains `interactive` + `providerOverride`; runtime honors the override

**Files:**
- Modify: `packages/core/src/llm/index.ts` (the `LLMRequest` interface ~line 51; `resolveRuntimeProvider` ~line 1939)
- Test: `packages/core/src/__tests__/llm.test.ts`

- [ ] **Step 1: Write the failing test** — append inside the existing `describe('Per-agent provider routing via identityResolver', ...)` block in `llm.test.ts`:

```ts
it('honors request.providerOverride above identity and global default', async () => {
  const tmpDirO = mkdtempSync(join(tmpdir(), 'fort-override-'));
  const { LLMProviderStore } = await import('../llm/provider-store.js');
  const store = new LLMProviderStore(join(tmpDirO, 'pstore.db'), 'test-key');
  store.addProvider({ id: 'anthropic', name: 'Anthropic', defaultModel: 'claude-sonnet-4-5-20250929', apiKey: 'sk-ant-stored', isDefault: true });
  store.addProvider({ id: 'openai', name: 'OpenAI', defaultModel: 'gpt-5.4', apiKey: 'sk-openai-stored', baseUrl: 'https://api.openai.com/v1' });

  const client = setup({ providerStore: store });
  // Default → anthropic; override forces openai for this one call.
  expect((client as any).resolveRuntimeProvider(undefined, 'openai')?.id).toBe('openai');
  expect((client as any).resolveRuntimeProvider(undefined, undefined)?.id).toBe('anthropic');

  store.close();
  rmSync(tmpDirO, { recursive: true, force: true });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/core && npx vitest run src/__tests__/llm.test.ts -t "providerOverride"`
Expected: FAIL — `resolveRuntimeProvider` takes one arg; override ignored.

- [ ] **Step 3: Add the request fields.** In `LLMRequest` (after `agentId?: string;`, ~line 58):

```ts
  /** When true, a gated 429 throws ModelGatedError instead of auto-falling-back.
   *  Set by the Specialist for interactive (user_chat) tasks. */
  interactive?: boolean;
  /** Forces a specific provider id for this one request, above identity/global
   *  resolution. Used to retry after the user picks a different provider. */
  providerOverride?: string;
```

- [ ] **Step 4: Honor the override in `resolveRuntimeProvider`.** Change the signature and add an override branch at the very top of the method (currently `private resolveRuntimeProvider(agentId?: string): RuntimeProvider | null {`):

```ts
  private resolveRuntimeProvider(agentId?: string, providerOverride?: string): RuntimeProvider | null {
    // Explicit per-request override wins (user picked a provider in the choice gate).
    if (providerOverride && this.providerStore) {
      const rt = this.providerStore.getProviderRuntime(providerOverride);
      if (rt) {
        const built = this.buildRuntimeProviderFromStore(rt);
        if (built) return built;
      }
    }
    if (this.providerStore) {
      const provider = this.getActiveProvider(agentId);
      // ... unchanged body ...
```

Then update the internal callers that should respect the override — `complete`, `completeWithTools`, and their OpenAI variants pass `request.providerOverride`. Find each `this.resolveRuntimeProvider(request.agentId)` and `this.resolveClient(request.agentId)` on the request path and change to `this.resolveRuntimeProvider(request.agentId, request.providerOverride)`. (Diagnostics/status callers with no request stay one-arg.)

- [ ] **Step 5: Run test to verify it passes**

Run: `cd packages/core && npx vitest run src/__tests__/llm.test.ts -t "providerOverride"`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add packages/core/src/llm/index.ts packages/core/src/__tests__/llm.test.ts
git commit -m "feat(llm): per-request providerOverride + interactive flag"
```

---

## Task 2: `ModelGatedError`, thrown on a gated 429 when `interactive`

**Files:**
- Modify: `packages/core/src/llm/index.ts` (export the error; the `complete()` catch ~line 725; the Anthropic tool-loop catch ~line 896; the OpenAI tool-loop catch ~line 2567)
- Test: `packages/core/src/__tests__/llm.test.ts`

- [ ] **Step 1: Write the failing test** — new `describe` block in `llm.test.ts`. It drives the plain `complete()` path: a provider-store-backed Anthropic client whose model is in cooldown, with `interactive: true`, must throw `ModelGatedError`; with `interactive` unset it must NOT (it auto-falls-back / errors differently).

```ts
describe('ModelGatedError on gated models (interactive)', () => {
  it('throws ModelGatedError when interactive and the tier is gated', async () => {
    const tmp = mkdtempSync(join(tmpdir(), 'fort-gated-'));
    const { LLMProviderStore } = await import('../llm/provider-store.js');
    const { ModelGatedError } = await import('../llm/index.js');
    const store = new LLMProviderStore(join(tmp, 'p.db'), 'k');
    store.addProvider({ id: 'anthropic', name: 'Anthropic', defaultModel: 'claude-opus-4-6', apiKey: 'sk-ant-x', isDefault: true });
    store.addProvider({ id: 'openai', name: 'OpenAI', defaultModel: 'gpt-5.4', apiKey: 'sk-openai-x', baseUrl: 'https://api.openai.com/v1' });

    const client = setup({ providerStore: store });
    // Put the powerful model into cooldown so the next call short-circuits to fallback.
    (client as any).setCooldown('claude-opus-4-6', 60_000, 'rate_limit');

    await expect(
      client.complete({ messages: [{ role: 'user', content: 'hi' }], model: 'powerful', interactive: true }),
    ).rejects.toBeInstanceOf(ModelGatedError);

    store.close();
    rmSync(tmp, { recursive: true, force: true });
  });

  it('auto-falls-back (no ModelGatedError) when not interactive', async () => {
    const tmp = mkdtempSync(join(tmpdir(), 'fort-gated-'));
    const { LLMProviderStore } = await import('../llm/provider-store.js');
    const { ModelGatedError } = await import('../llm/index.js');
    const store = new LLMProviderStore(join(tmp, 'p.db'), 'k');
    store.addProvider({ id: 'anthropic', name: 'Anthropic', defaultModel: 'claude-opus-4-6', apiKey: 'sk-ant-x', isDefault: true });
    const client = setup({ providerStore: store });
    (client as any).setCooldown('claude-opus-4-6', 60_000, 'rate_limit');
    // Stub the actual HTTP so the fallback tier "succeeds" deterministically.
    vi.spyOn(client as any, 'callApi').mockResolvedValue({
      content: [{ type: 'text', text: 'ok' }], stop_reason: 'end_turn',
      usage: { input_tokens: 1, output_tokens: 1 }, model: 'claude-haiku-4-5-20251001',
    });
    const res = await client.complete({ messages: [{ role: 'user', content: 'hi' }], model: 'powerful' });
    expect(res.content).toBe('ok');

    store.close();
    rmSync(tmp, { recursive: true, force: true });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/core && npx vitest run src/__tests__/llm.test.ts -t "ModelGatedError"`
Expected: FAIL — `ModelGatedError` is not exported.

- [ ] **Step 3: Define `ModelGatedError`.** Add near the top of `llm/index.ts` (after the `IdentityResolver` type, ~line 95):

```ts
/** Thrown (only when request.interactive) when the requested model is gated by
 *  a 429 and a tier/provider fallback exists. The Specialist turns this into a
 *  user-facing choice card. */
export class ModelGatedError extends Error {
  constructor(
    public readonly gatedModel: string,
    public readonly gatedTier: ModelTier,
    public readonly providerId: string,
    public readonly viableProviders: Array<{ id: string; name: string; powerfulModel: string }>,
    public readonly viableTiers: ModelTier[],
    public readonly canUseApiKey: boolean,
  ) {
    super('__MODEL_GATED__');
    this.name = 'ModelGatedError';
  }
}
```

- [ ] **Step 4: Add a helper to build the error**, as a private method on `LLMClient` (near `getNextAvailableTier`, ~line 1642):

```ts
  /** Build a ModelGatedError describing the alternatives to a gated model. */
  private async buildModelGatedError(modelConfig: ModelConfig, providerId: string): Promise<ModelGatedError> {
    const all = await this.getAvailableProviders();
    const viableProviders = all
      .filter((p) => p.usable && p.id !== providerId)
      .map((p) => ({ id: p.id, name: p.name, powerfulModel: p.models.powerful }));
    const viableTiers: ModelTier[] = [];
    for (const t of ['standard', 'fast'] as ModelTier[]) {
      if (TIER_FALLBACK.indexOf(t) > TIER_FALLBACK.indexOf(modelConfig.tier)
          && !this.isInCooldown(this.models[t].model)) {
        viableTiers.push(t);
      }
    }
    const canUseApiKey = providerId === 'anthropic' && this._isOAuthToken;
    return new ModelGatedError(modelConfig.model, modelConfig.tier, providerId, viableProviders, viableTiers, canUseApiKey);
  }
```

- [ ] **Step 5: Throw it at the `complete()` catch.** Replace the `__TIER_FALLBACK__` branch at ~line 725:

```ts
    } catch (err: any) {
      if (err?.message === '__TIER_FALLBACK__' && err._fallbackTier) {
        if (request.interactive) {
          const runtime = this.resolveRuntimeProvider(request.agentId, request.providerOverride);
          throw await this.buildModelGatedError(modelConfig, runtime?.id ?? 'anthropic');
        }
        return this.complete({ ...request, model: err._fallbackTier }, (err._fallbackDepth ?? 0) + 1);
      }
      throw err;
    }
```

- [ ] **Step 6: Throw it at the Anthropic tool-loop catch** (~line 896, the block added in v0.3.9 that switches `modelConfig`). At the top of the `if (err?.message === '__TIER_FALLBACK__' && err._fallbackTier)` body, before the inline switch:

```ts
          if (err?.message === '__TIER_FALLBACK__' && err._fallbackTier) {
            if (request.interactive) {
              throw await this.buildModelGatedError(modelConfig, runtime.id);
            }
            const fallback = this.resolveModelForProvider(runtime, err._fallbackTier);
            // ... existing inline-switch body unchanged ...
```

- [ ] **Step 7: Throw it at the OpenAI tool-loop catch** (~line 2567). Same guard at the top of the `__TIER_FALLBACK__` branch:

```ts
          if (err?.message === '__TIER_FALLBACK__' && err._fallbackTier) {
            if (request.interactive) {
              throw await this.buildModelGatedError(modelConfig, currentProvider.id);
            }
            const fallback = this.resolveModelForProvider(currentProvider, err._fallbackTier);
            // ... existing inline-switch body unchanged ...
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `cd packages/core && npx vitest run src/__tests__/llm.test.ts -t "ModelGatedError"`
Expected: PASS (both cases)

- [ ] **Step 9: Commit**

```bash
git add packages/core/src/llm/index.ts packages/core/src/__tests__/llm.test.ts
git commit -m "feat(llm): throw ModelGatedError on gated 429 for interactive requests"
```

---

## Task 3: `ModelChoiceService` (block + resolve, modeled on awaitApproval)

**Files:**
- Create: `packages/core/src/services/model-choice.ts`
- Test: `packages/core/src/__tests__/model-choice.test.ts`

- [ ] **Step 1: Write the failing test** — `packages/core/src/__tests__/model-choice.test.ts`:

```ts
import { describe, it, expect, vi } from 'vitest';
import { ModuleBus } from '../module-bus/index.js';
import { ModelChoiceService } from '../services/model-choice.js';

describe('ModelChoiceService', () => {
  it('publishes model-choice.required and resolves when answered', async () => {
    const bus = new ModuleBus();
    const svc = new ModelChoiceService(bus);
    const events: any[] = [];
    bus.subscribe('model-choice.required', (e) => { events.push(e.payload); });

    const p = svc.requestChoice({
      taskId: 't1', agentId: 'fort', gatedModel: 'claude-opus-4-6',
      options: [{ action: 'switch_provider', providerId: 'openai', label: 'Switch to OpenAI' }],
    });
    expect(events).toHaveLength(1);
    const id = events[0].id;
    svc.resolveChoice(id, { action: 'switch_provider', providerId: 'openai', remember: false });
    await expect(p).resolves.toEqual({ action: 'switch_provider', providerId: 'openai', remember: false });
  });

  it('times out to a fallback action', async () => {
    vi.useFakeTimers();
    const bus = new ModuleBus();
    const svc = new ModelChoiceService(bus);
    const p = svc.requestChoice({ taskId: 't2', agentId: 'fort', gatedModel: 'm', options: [] });
    vi.advanceTimersByTime(600_001);
    await expect(p).resolves.toEqual({ action: 'fallback', remember: false });
    vi.useRealTimers();
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/core && npx vitest run src/__tests__/model-choice.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Implement the service** — `packages/core/src/services/model-choice.ts`:

```ts
import { randomUUID } from 'node:crypto';
import type { ModuleBus } from '../module-bus/index.js';
import type { AgentFactory } from '../agents/hatchery.js';

export type ChoiceOption =
  | { action: 'switch_provider'; providerId: string; label: string }
  | { action: 'lighter_model'; tier: 'fast' | 'standard'; label: string }
  | { action: 'use_api_key'; providerId: string; label: string };

export interface ChoiceRequest {
  taskId: string;
  agentId: string;
  gatedModel: string;
  options: ChoiceOption[];
}

export interface ResolvedChoice {
  action: 'switch_provider' | 'lighter_model' | 'use_api_key' | 'fallback';
  providerId?: string;
  tier?: 'fast' | 'standard';
  apiKey?: string;
  remember: boolean;
}

const TIMEOUT_MS = 600_000; // 10 minutes — matches tool approval

/**
 * Blocks an interactive task while the user picks how to handle a gated model.
 * Same shape as ToolExecutor.awaitApproval: a resolver Map + bus event + timeout.
 */
export class ModelChoiceService {
  private pending = new Map<string, { resolve: (c: ResolvedChoice) => void }>();
  private factory: AgentFactory | null = null;

  constructor(private bus: ModuleBus) {}

  /** Injected in Fort so remembered choices can be persisted to identity.yaml. */
  setAgentFactory(factory: AgentFactory): void { this.factory = factory; }

  requestChoice(req: ChoiceRequest): Promise<ResolvedChoice> {
    const id = randomUUID();
    void this.bus.publish('model-choice.required', 'model-choice', { id, ...req });
    return new Promise<ResolvedChoice>((resolve) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        resolve({ action: 'fallback', remember: false });
      }, TIMEOUT_MS);
      this.pending.set(id, {
        resolve: (c) => { clearTimeout(timer); this.pending.delete(id); resolve(c); },
      });
    });
  }

  resolveChoice(id: string, choice: ResolvedChoice): boolean {
    const p = this.pending.get(id);
    if (!p) return false;
    p.resolve(choice);
    return true;
  }

  /** Persist a remembered choice to the agent's identity. No-op without a factory. */
  persist(agentId: string, patch: { provider?: string; defaultModelTier?: 'fast' | 'standard' | 'powerful' }): void {
    this.factory?.updateIdentity(agentId, patch as any);
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd packages/core && npx vitest run src/__tests__/model-choice.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add packages/core/src/services/model-choice.ts packages/core/src/__tests__/model-choice.test.ts
git commit -m "feat(core): ModelChoiceService — block on user choice for gated models"
```

---

## Task 4: Wire the service into Fort + AgentFactory + Specialist DI

**Files:**
- Modify: `packages/core/src/fort.ts` (near the hatch wiring ~line 266)
- Modify: `packages/core/src/agents/hatchery.ts` (add `setModelChoice`, pass to specialists at the 5 `agent.setLLM` sites: ~133, ~194, ~282, ~324, ~492)
- Modify: `packages/core/src/agents/specialist.ts` (add field + `setModelChoice`)

- [ ] **Step 1: Add the field + setter to Specialist.** In `specialist.ts` after the `toolExecutor` field (~line 30):

```ts
  private modelChoice: import('../services/model-choice.js').ModelChoiceService | null = null;
```

And after `setToolExecutor` (~line 114):

```ts
  setModelChoice(svc: import('../services/model-choice.js').ModelChoiceService): void {
    this.modelChoice = svc;
  }
```

- [ ] **Step 2: Add factory plumbing.** In `hatchery.ts`, add a field + setter mirroring `setToolExecutor`:

```ts
  private modelChoice: import('../services/model-choice.js').ModelChoiceService | null = null;

  setModelChoice(svc: import('../services/model-choice.js').ModelChoiceService): void {
    this.modelChoice = svc;
  }
```

Then at EACH of the five places that do `if (this.toolExecutor) agent.setToolExecutor(this.toolExecutor);`, add the sibling line:

```ts
      if (this.modelChoice) agent.setModelChoice(this.modelChoice);
```

- [ ] **Step 3: Construct + wire in Fort.** In `fort.ts`, after `this.hatch = new HatchService(...)` (~line 266):

```ts
    const { ModelChoiceService } = await import('./services/model-choice.js');
    this.modelChoice = new ModelChoiceService(this.bus);
    this.modelChoice.setAgentFactory(this.agentFactory);
    this.agentFactory.setModelChoice(this.modelChoice);
```

Add the field declaration alongside the other Fort members (e.g. near `hatch`): `modelChoice!: import('./services/model-choice.js').ModelChoiceService;`. If `fort.ts`'s constructor is not async, import `ModelChoiceService` at the top of the file instead of `await import` and drop the dynamic import.

- [ ] **Step 4: Build to verify wiring compiles**

Run: `npm run build`
Expected: PASS (no type errors)

- [ ] **Step 5: Commit**

```bash
git add packages/core/src/fort.ts packages/core/src/agents/hatchery.ts packages/core/src/agents/specialist.ts
git commit -m "feat(core): wire ModelChoiceService into Fort, factory, specialists"
```

---

## Task 5: Specialist runs the gate on `ModelGatedError` and retries

**Files:**
- Modify: `packages/core/src/agents/specialist.ts` (the LLM call block, ~lines 257-315)
- Test: `packages/core/src/__tests__/specialist-gate.test.ts`

- [ ] **Step 1: Write the failing test** — `packages/core/src/__tests__/specialist-gate.test.ts`. It stubs an LLM that throws `ModelGatedError` once then succeeds, a `ModelChoiceService` auto-answering "switch_provider openai, remember", and asserts the task completes and the identity was persisted.

```ts
import { describe, it, expect, vi } from 'vitest';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { Fort } from '../fort.js';
import { ModelGatedError } from '../llm/index.js';

describe('Specialist gated-model gate', () => {
  it('runs the choice gate on ModelGatedError and retries on the chosen provider', async () => {
    const dir = mkdtempSync(join(tmpdir(), 'fort-gate-'));
    const fort = new Fort({ dataDir: join(dir, 'data'), agentsDir: join(dir, 'agents') } as any);
    await fort.start();
    const agent = fort.agentFactory.create({ name: 'Fort' });
    agent.identity.isDefault = true;
    await agent.start();

    // First call throws gated; retry (providerOverride='openai') succeeds.
    let calls = 0;
    vi.spyOn(fort.llm, 'completeWithTools').mockImplementation(async (req: any) => {
      calls++;
      if (calls === 1) throw new ModelGatedError('claude-opus-4-6', 'powerful', 'anthropic',
        [{ id: 'openai', name: 'OpenAI', powerfulModel: 'gpt-5.5' }], ['fast'], false);
      return { content: 'hello from openai', model: 'gpt-5.5', inputTokens: 1, outputTokens: 1, totalTokens: 2, costUsd: 0, stopReason: 'end_turn', durationMs: 1, toolCallLog: [], iterations: 1 } as any;
    });
    // Auto-answer the gate.
    fort.bus.subscribe('model-choice.required', (e: any) => {
      fort.modelChoice.resolveChoice(e.payload.id, { action: 'switch_provider', providerId: 'openai', remember: true });
    });

    const task = await fort.chat('hi', 'user_chat', agent.identity.id);
    expect(fort.taskGraph.getTask(task.id).status).toBe('completed');
    expect(fort.agentFactory.getIdentity(agent.identity.id)?.provider).toBe('openai');

    await fort.stop();
    rmSync(dir, { recursive: true, force: true });
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/core && npx vitest run src/__tests__/specialist-gate.test.ts`
Expected: FAIL — Specialist doesn't catch `ModelGatedError` yet.

- [ ] **Step 3: Implement the gate + retry loop in `Specialist.execute`.** Replace the LLM-call block (currently the `if (this.llm && this.llm.isConfigured && isChatTask) { ... }` around lines 257-315) so the call is wrapped in a bounded retry that handles `ModelGatedError`. Add `import { ModelGatedError } from '../llm/index.js';` at the top of `specialist.ts`.

```ts
    if (this.llm && this.llm.isConfigured && isChatTask) {
      const soul = this.getSoul();
      const baseTier = (task.metadata.modelTier as string) || this.identity.defaultModelTier;
      const liveTools = this.toolRegistry ? this.toolRegistry.listLiveTools() : [];
      const hatchMode = this.identity.hatchedAt == null;
      const interactive = task.source === 'user_chat';

      let providerOverride: string | undefined;
      let tierOverride: string | undefined;
      let triedGated = new Set<string>(); // models we've already been told are gated
      let rounds = 0;

      while (true) {
        rounds++;
        const req = {
          messages: [{ role: 'user' as const, content: task.description }],
          soul: soul ?? undefined,
          taskId: task.id,
          agentId: this.identity.id,
          model: tierOverride ?? baseTier,
          providerOverride,
          injectBehaviors: true,
          injectMemory: task.description,
          context: toolContext ? [toolContext] : undefined,
          hatchMode,
          interactive,
        };
        try {
          if (this.toolExecutor && liveTools.length > 0) {
            const response = await this.llm.completeWithTools({ ...req, tools: liveTools }, this.toolExecutor);
            responseText = response.content;
            if (response.toolCallLog.length > 0) {
              task.metadata.toolCallLog = response.toolCallLog;
              task.metadata.toolIterations = response.iterations;
            }
          } else {
            const response = await this.llm.complete(req);
            responseText = response.content;
          }
          break; // success
        } catch (err) {
          if (err instanceof ModelGatedError && this.modelChoice && rounds <= 4) {
            triedGated.add(err.gatedModel);
            this.taskGraph.updateStatus(task.id, 'blocked', 'Model gated — awaiting your choice');
            const options = [
              ...err.viableProviders.map((p) => ({ action: 'switch_provider' as const, providerId: p.id, label: `Switch to ${p.name}` })),
              ...err.viableTiers.map((t) => ({ action: 'lighter_model' as const, tier: t as 'fast' | 'standard', label: `Use a lighter ${err.providerId} model` })),
              ...(err.canUseApiKey ? [{ action: 'use_api_key' as const, providerId: err.providerId, label: `Use ${err.providerId} API key instead` }] : []),
            ];
            const choice = await this.modelChoice.requestChoice({ taskId: task.id, agentId: this.identity.id, gatedModel: err.gatedModel, options });
            this.taskGraph.updateStatus(task.id, 'in_progress', 'Resuming after model choice');

            if (choice.action === 'switch_provider' && choice.providerId) {
              providerOverride = choice.providerId; tierOverride = undefined;
              if (choice.remember) this.modelChoice.persist(this.identity.id, { provider: choice.providerId });
            } else if (choice.action === 'lighter_model' && choice.tier) {
              tierOverride = choice.tier; providerOverride = undefined;
              if (choice.remember) this.modelChoice.persist(this.identity.id, { defaultModelTier: choice.tier });
            } else if (choice.action === 'use_api_key' && choice.apiKey) {
              const { LLMClient } = await import('../llm/index.js');
              const envVar = err.providerId === 'anthropic' ? 'ANTHROPIC_API_KEY' : 'OPENAI_API_KEY';
              if (err.providerId === 'anthropic') LLMClient.writeEnvFile(choice.apiKey);
              else LLMClient.writeEnvFileValue(envVar, choice.apiKey);
              this.llm.refreshAuth();
              providerOverride = err.providerId; tierOverride = undefined;
            } else {
              // fallback / timeout — degrade to the lowest viable tier and stop asking
              tierOverride = err.viableTiers[err.viableTiers.length - 1] ?? 'fast';
              providerOverride = undefined;
            }
            continue;
          }
          // Not gated, no service, or out of rounds — fall through to existing error handling.
          const msg = err instanceof Error ? err.message : String(err);
          if (msg.includes('401') || msg.includes('authentication_error') || msg.includes('Invalid bearer')) {
            responseText = `Authentication error — your Claude token may be expired. Run \`fort llm setup\` or \`claude setup-token\` to re-authenticate, then restart the portal.`;
          } else {
            responseText = `I encountered an error: ${msg}. Please try again.`;
          }
          break;
        }
      }
    } else if (isChatTask) {
```

(Keep the rest of the method — the `else if (isChatTask)` / `else` branches and everything after — unchanged.)

- [ ] **Step 4: Run test to verify it passes**

Run: `cd packages/core && npx vitest run src/__tests__/specialist-gate.test.ts`
Expected: PASS

- [ ] **Step 5: Full core suite — no regressions**

Run: `cd packages/core && npx vitest run`
Expected: only the 3 known pre-existing failures (auth.test.ts ×2, hatchery.test.ts "should start created agent and handle tasks").

- [ ] **Step 6: Commit**

```bash
git add packages/core/src/agents/specialist.ts packages/core/src/__tests__/specialist-gate.test.ts
git commit -m "feat(agent): gated-model choice gate with bounded retry in Specialist"
```

---

## Task 6: Server bus↔WS bridge + `model-choice.respond` handler

**Files:**
- Modify: `packages/core/src/server/index.ts` (bus bridge near the `approval.required` subscription ~line 277; new WS `case` near `approval.respond` ~line 1288)

- [ ] **Step 1: Bridge the bus event to WS.** Next to the `approval.required` subscription (~line 277):

```ts
    this.fort.bus.subscribe('model-choice.required', (event) => {
      this.broadcast({ id: event.id, type: 'model-choice.new', payload: event.payload });
    });
```

- [ ] **Step 2: Handle the client response.** Add a `case` in `handleMessage` next to `approval.respond` (~line 1288):

```ts
      case 'model-choice.respond': {
        const p = (msg.payload ?? {}) as {
          id: string;
          action: 'switch_provider' | 'lighter_model' | 'use_api_key' | 'fallback';
          providerId?: string;
          tier?: 'fast' | 'standard';
          apiKey?: string;
          remember?: boolean;
        };
        const ok = this.fort.modelChoice.resolveChoice(p.id, {
          action: p.action,
          providerId: p.providerId,
          tier: p.tier,
          apiKey: p.apiKey,
          remember: p.remember ?? false,
        });
        return { id: msg.id, type: 'model-choice.respond.response', payload: { ok } };
      }
```

- [ ] **Step 3: Build to verify it compiles**

Run: `npm run build`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add packages/core/src/server/index.ts
git commit -m "feat(server): model-choice WS bridge + respond handler"
```

---

## Task 7: Dashboard choice card + round-trip

**Files:**
- Modify: `packages/dashboard/src/types/index.ts` (extend `ChatMessage`)
- Modify: `packages/dashboard/src/pages/ChatPage.tsx` (subscribe + render + respond)

- [ ] **Step 1: Extend the message type.** In `types/index.ts`, change the `role` union and add a payload shape:

```ts
  role: "user" | "agent" | "tool" | "plan" | "classification" | "model-choice";
```

And add to the `ChatMessage` interface:

```ts
  /** Present on role:"model-choice" — the gated-model choice card. */
  modelChoice?: {
    id: string;
    gatedModel: string;
    options: Array<{ action: "switch_provider" | "lighter_model" | "use_api_key"; providerId?: string; tier?: "fast" | "standard"; label: string }>;
    resolved?: string; // one-line summary once answered
  };
```

- [ ] **Step 2: Subscribe to `model-choice.new`.** In `ChatPage.tsx`, inside the big `subscribe(...)` effect (the array around line 154-345), add:

```ts
      subscribe("model-choice.new", (msg: WSMessage) => {
        const p = msg.payload as { id: string; agentId?: string; gatedModel: string; options: any[] };
        const aid = p.agentId || selectedAgent;
        if (!aid) return;
        setChatMessages((prev) => ({
          ...prev,
          [aid]: [...(prev[aid] || []), { role: "model-choice", text: "", ts: Date.now(), modelChoice: { id: p.id, gatedModel: p.gatedModel, options: p.options } }],
        }));
      }),
```

- [ ] **Step 3: Render the card.** In the message render switch/map (where `role === "plan"` and `role === "classification"` are handled), add a branch for `model-choice`. Provide a small inline component:

```tsx
{m.role === "model-choice" && m.modelChoice && (
  <div className="model-choice-card">
    {m.modelChoice.resolved ? (
      <div className="model-choice-resolved">✓ {m.modelChoice.resolved}</div>
    ) : (
      <>
        <div className="model-choice-title">⚠ {m.modelChoice.gatedModel} is gated. How should I proceed?</div>
        {m.modelChoice.options.map((opt, i) => (
          <button key={i} className="model-choice-opt" onClick={() => respondModelChoice(m.modelChoice!, opt, rememberChoice)}>
            {opt.label}
          </button>
        ))}
        <label className="model-choice-remember">
          <input type="checkbox" checked={rememberChoice} onChange={(e) => setRememberChoice(e.target.checked)} /> Remember for this agent
        </label>
      </>
    )}
  </div>
)}
```

Add component state near the other `useState`s: `const [rememberChoice, setRememberChoice] = useState(false);`

- [ ] **Step 4: Implement `respondModelChoice`.** Add inside the component:

```ts
  const respondModelChoice = (
    mc: NonNullable<ChatMessage["modelChoice"]>,
    opt: NonNullable<ChatMessage["modelChoice"]>["options"][number],
    remember: boolean,
  ) => {
    // use_api_key with no key configured: prompt inline (simple window.prompt for v1).
    let apiKey: string | undefined;
    if (opt.action === "use_api_key") {
      apiKey = window.prompt(`Paste your ${opt.providerId} API key`) ?? undefined;
      if (!apiKey) return;
    }
    send("model-choice.respond", {
      id: mc.id, action: opt.action, providerId: opt.providerId, tier: opt.tier, apiKey, remember,
    });
    // Collapse the card to a summary.
    setChatMessages((prev) => {
      const aid = selectedAgent!;
      return {
        ...prev,
        [aid]: (prev[aid] || []).map((x) =>
          x.role === "model-choice" && x.modelChoice?.id === mc.id
            ? { ...x, modelChoice: { ...x.modelChoice!, resolved: opt.label } }
            : x),
      };
    });
    setRememberChoice(false);
  };
```

- [ ] **Step 5: Build the dashboard**

Run: `npm run build`
Expected: PASS (tsc + vite)

- [ ] **Step 6: Commit**

```bash
git add packages/dashboard/src/types/index.ts packages/dashboard/src/pages/ChatPage.tsx
git commit -m "feat(portal): gated-model choice card + WS round-trip"
```

---

## Task 8: End-to-end verification

- [ ] **Step 1: Build everything + run the core suite**

Run: `npm run build && cd packages/core && npx vitest run && cd ../..`
Expected: build clean; only the 3 known pre-existing failures.

- [ ] **Step 2: Manual portal check** (requires opus/sonnet currently gated on the Claude subscription, OpenAI configured):

```bash
PID=$(lsof -nP -iTCP:4077 -sTCP:LISTEN -t); [ -n "$PID" ] && kill $PID
node packages/cli/dist/index.js portal &   # or `fort portal` after deploy
open http://localhost:4077
```
- Send a chat to the primary (opus-tier) agent.
- Expect the card: "claude-opus-4-6 is gated. How should I proceed?" with **Switch to OpenAI**, **Use a lighter anthropic model**, **Use anthropic API key instead**, and a **Remember for this agent** checkbox.
- Pick **Switch to OpenAI** with remember checked → reply streams from gpt-5.x; `cat ~/.fort/agents/<id>/identity.yaml | grep provider` shows `provider: openai`.
- Verify a background path still auto-falls-back: trigger a scheduled/decomposed task and confirm no card and no stall (check `~/.fort` logs / token tracker for a haiku-tier completion).

- [ ] **Step 3: Commit any fixes from manual testing** (if needed).

---

## Task 9: Release v0.4.0

This is a user-visible feature → minor bump. Use the `cut-fort-release` skill at `~/.claude/skills/cut-fort-release/SKILL.md`.

- [ ] **Step 1:** Bump root + core + cli (+ `@fort-ai/core` dep) + dashboard package.json to `0.4.0`; `npm install --package-lock-only`.
- [ ] **Step 2:** Commit `chore(release): v0.4.0`; push; tag `v0.4.0`; push tag.
- [ ] **Step 3:** `gh release create v0.4.0` with notes summarizing the choice gate.
- [ ] **Step 4:** Recompute tarball sha256; update `tobsai/homebrew-tap/Formula/fort.rb` (url + sha256) via `gh api ... -X PUT`.
- [ ] **Step 5:** `brew update && brew upgrade fort && fort --version` → `0.4.0`.

---

## Self-review notes

- **Spec coverage:** trigger/scope (Task 5 `interactive`), ModelGatedError (Task 2), choice gate block/resolve/timeout (Task 3), dynamic options incl. lighter + provider + api-key (Task 5 builds options, Task 7 renders), remember=permanent persist (Task 5 `persist` → `updateIdentity`), per-task unremembered (no session memory — overrides are local vars reset each task), lighter-also-gated re-show (Task 5 loop re-throws with smaller option set + `triedGated`; `getNextAvailableTier`/`viableTiers` exclude cooled-down models), background auto-fallback retained (Task 2 guards on `request.interactive`), `blocked` status (Task 5), WS bridge (Task 6), card (Task 7).
- **Open spec item (use_api_key remember):** resolved as "the written key is the durable signal" — Task 5 writes the key + `refreshAuth()` and does NOT add an identity field. No `authPreference` field introduced.
- **Type consistency:** `ResolvedChoice`/`ChoiceOption` shapes match across service (Task 3), specialist (Task 5), server (Task 6), and dashboard payload (Task 7). `viableTiers` typed `ModelTier[]` but narrowed to `'fast' | 'standard'` in options (powerful is never a fallback target).
- **Mid-loop limitation:** documented in spec; Task 5 retries the whole call (acceptable for v1 — gating hits the first call in the common case).
