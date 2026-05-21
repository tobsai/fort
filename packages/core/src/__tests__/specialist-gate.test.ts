import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, mkdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { Fort } from '../fort.js';
import { ModelGatedError } from '../llm/index.js';

/**
 * Spin up an isolated Fort + a hatched default agent in a temp dir.
 * Returns the started Fort, the agent, and a cleanup() that stops Fort and
 * removes the temp dir. specsDir is created so spec-driven paths don't blow up.
 */
async function makeFort() {
  const dir = mkdtempSync(join(tmpdir(), 'fort-gate-'));
  const specsDir = join(dir, 'specs');
  mkdirSync(specsDir, { recursive: true });
  const fort = new Fort({ dataDir: join(dir, 'data'), specsDir, agentsDir: join(dir, 'agents') } as any);
  await fort.start();
  const agent = fort.agentFactory.create({ name: 'Fort' });
  agent.identity.isDefault = true;
  // Hatched so the LLM call runs in normal mode (not the hatch onboarding flow).
  agent.identity.hatchedAt = new Date().toISOString();
  await agent.start();
  const cleanup = async () => {
    await fort.stop();
    rmSync(dir, { recursive: true, force: true });
  };
  return { fort, agent, cleanup };
}

describe('Specialist gated-model gate', () => {
  let prevTriage: string | undefined;
  beforeEach(() => {
    // Skip the classify/decompose LLM call so the FIRST live LLM call is the
    // response-path call the gate wraps (otherwise classify would consume the
    // mocked gated error and swallow it in its own catch).
    prevTriage = process.env.FORT_DISABLE_TRIAGE;
    process.env.FORT_DISABLE_TRIAGE = '1';
  });
  afterEach(() => {
    if (prevTriage === undefined) delete process.env.FORT_DISABLE_TRIAGE;
    else process.env.FORT_DISABLE_TRIAGE = prevTriage;
  });

  it('runs the choice gate on ModelGatedError and retries on the chosen provider', async () => {
    const { fort, agent, cleanup } = await makeFort();

    // First call throws gated; retry (providerOverride='openai') succeeds.
    let calls = 0;
    const stub = async (req: any) => {
      calls++;
      if (calls === 1) throw new ModelGatedError('claude-opus-4-6', 'powerful', 'anthropic',
        [{ id: 'openai', name: 'OpenAI', powerfulModel: 'gpt-5.5' }], ['fast'], false);
      return { content: 'hello from openai', model: 'gpt-5.5', inputTokens: 1, outputTokens: 1, totalTokens: 2, costUsd: 0, stopReason: 'end_turn', durationMs: 1, toolCallLog: [], iterations: 1 } as any;
    };
    vi.spyOn(fort.llm, 'completeWithTools').mockImplementation(stub as any);
    vi.spyOn(fort.llm, 'complete').mockImplementation(stub as any);
    // Stub the strict-completion reviewer (taskGraph.reviewCompletion → llm.ask)
    // so it doesn't consume a gate-path call. Approve so the task completes.
    vi.spyOn(fort.llm, 'ask').mockResolvedValue('{"approved": true, "reason": "ok"}');

    // Capture the status sequence so we can prove the task passed through
    // 'blocked' while the gate was open. newStatus is a primitive captured at
    // publish time (task is a live reference that gets mutated afterward), so
    // it's the reliable field to read.
    const statuses: string[] = [];
    fort.bus.subscribe('task.status_changed', (e: any) => {
      const status = e.payload?.newStatus ?? e.payload?.task?.status ?? e.payload?.status;
      if (status) statuses.push(status);
    });

    // Auto-answer the gate.
    fort.bus.subscribe('model-choice.required', (e: any) => {
      fort.modelChoice.resolveChoice(e.payload.id, { action: 'switch_provider', providerId: 'openai', remember: true });
    });

    const task = await fort.chat('hi', 'user_chat', agent.identity.id);
    expect(fort.taskGraph.getTask(task.id).status).toBe('completed');
    expect(fort.agentFactory.getIdentity(agent.identity.id)?.provider).toBe('openai');
    // A retry actually happened: first call gated, second succeeded.
    expect(calls).toBe(2);
    // The task was blocked while awaiting the user's choice.
    expect(statuses).toContain('blocked');

    await cleanup();
  });

  it('degrades to a viable lower tier when the choice falls back / times out', async () => {
    const { fort, agent, cleanup } = await makeFort();

    // First call throws gated (only 'fast' is viable); retry succeeds. The
    // gate resolves to 'fallback', which should degrade to the lowest viable
    // tier ('fast') WITHOUT switching providers or persisting anything.
    let calls = 0;
    const stub = async (req: any) => {
      calls++;
      if (calls === 1) throw new ModelGatedError('claude-opus-4-6', 'powerful', 'anthropic',
        [], ['fast'], false);
      return { content: 'hello from fast tier', model: 'claude-haiku', inputTokens: 1, outputTokens: 1, totalTokens: 2, costUsd: 0, stopReason: 'end_turn', durationMs: 1, toolCallLog: [], iterations: 1 } as any;
    };
    vi.spyOn(fort.llm, 'completeWithTools').mockImplementation(stub as any);
    vi.spyOn(fort.llm, 'complete').mockImplementation(stub as any);
    // Stub the strict-completion reviewer so it doesn't consume a gate-path call.
    vi.spyOn(fort.llm, 'ask').mockResolvedValue('{"approved": true, "reason": "ok"}');

    // Resolve the gate with a fallback (the deterministic stand-in for a
    // timeout — requestChoice's timer resolves to the same shape).
    fort.bus.subscribe('model-choice.required', (e: any) => {
      fort.modelChoice.resolveChoice(e.payload.id, { action: 'fallback', remember: false });
    });

    const task = await fort.chat('hi', 'user_chat', agent.identity.id);
    // Task still completed by degrading to the lower tier.
    expect(fort.taskGraph.getTask(task.id).status).toBe('completed');
    expect(calls).toBe(2);
    // Fallback never persists — provider stays undefined.
    expect(fort.agentFactory.getIdentity(agent.identity.id)?.provider).toBeUndefined();

    await cleanup();
  });
});
