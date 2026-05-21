import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, mkdirSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { Fort } from '../fort.js';
import { ModelGatedError } from '../llm/index.js';

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
