import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { LLMClient } from '../llm/index.js';
import { ModuleBus } from '../module-bus/index.js';
import { TokenTracker } from '../tokens/index.js';
import { BehaviorManager } from '../behaviors/index.js';
import { MemoryManager } from '../memory/index.js';

// `buildSystemPrompt` is private. Call it via `(client as any)` from tests.
// Snapshot tests pin the assembled prompt so any drift in voice or section
// ordering is visible in review.

describe('LLMClient.buildSystemPrompt — voice and assembly', () => {
  let tmpDir: string;
  let bus: ModuleBus;
  let tokens: TokenTracker;
  let memory: MemoryManager;
  let behaviors: BehaviorManager;
  let client: LLMClient;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-llm-prompt-'));
    bus = new ModuleBus();
    tokens = new TokenTracker(join(tmpDir, 'tokens.db'), bus);
    memory = new MemoryManager(join(tmpDir, 'memory.db'), bus);
    behaviors = new BehaviorManager(memory, bus);
    vi.spyOn(LLMClient, 'readEnvFile').mockReturnValue(null);
    vi.spyOn(LLMClient, 'readOpenAIEnvFile').mockReturnValue(null);
    vi.spyOn(LLMClient, 'readCodexOpenAIToken').mockReturnValue(null);
    vi.spyOn(LLMClient, 'readKeychainToken').mockReturnValue(null);
    client = new LLMClient({}, bus, tokens, behaviors, memory);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    tokens.close();
    memory.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  function stripTime(s: string): string {
    // Current Time section is dynamic; strip it for stable snapshots.
    return s.replace(/## Current Time\n[^\n]*/g, '## Current Time\n<TIMESTAMP>');
  }

  it('default base prompt defines quick and curious modes', async () => {
    const prompt = await (client as any).buildSystemPrompt({
      messages: [{ role: 'user', content: 'hello' }],
    });
    expect(prompt).toMatch(/Quick mode/);
    expect(prompt).toMatch(/Curious mode/);
    expect(prompt).toMatch(/No inner monologue/);
    expect(prompt).toMatch(/One clarifying question at a time/i);
    expect(prompt).toMatch(/Speak personally/);
  });

  it('does not say "prefer to take action rather than ask"', async () => {
    // Guards against regression to the old action-oriented prompt.
    const prompt = await (client as any).buildSystemPrompt({
      messages: [{ role: 'user', content: 'hello' }],
    });
    expect(prompt).not.toMatch(/prefer to take action rather than ask/i);
  });

  it('renders the bare prompt without soul, behaviors, memory, or context', async () => {
    const prompt = await (client as any).buildSystemPrompt({
      messages: [{ role: 'user', content: 'hello' }],
      injectBehaviors: false,
    });
    expect(stripTime(prompt)).toMatchSnapshot();
  });

  it('injects soul under "## Agent Identity"', async () => {
    const prompt = await (client as any).buildSystemPrompt({
      messages: [{ role: 'user', content: 'hello' }],
      injectBehaviors: false,
      soul: '# Coach\n\nYou are a fitness coach.',
    });
    expect(prompt).toMatch(/## Agent Identity\n# Coach/);
    expect(stripTime(prompt)).toMatchSnapshot();
  });

  it('honors an explicit `system` override (e.g. Triager, Hatch)', async () => {
    const prompt = await (client as any).buildSystemPrompt({
      messages: [{ role: 'user', content: 'hello' }],
      injectBehaviors: false,
      system: 'OVERRIDDEN_BASE_PROMPT',
    });
    expect(prompt.startsWith('OVERRIDDEN_BASE_PROMPT')).toBe(true);
    expect(prompt).not.toMatch(/Quick mode/);
  });

  it('appends additional context block when provided', async () => {
    const prompt = await (client as any).buildSystemPrompt({
      messages: [{ role: 'user', content: 'hello' }],
      injectBehaviors: false,
      context: ['Project: Fort', 'User timezone: America/Los_Angeles'],
    });
    expect(prompt).toMatch(/## Additional Context\nProject: Fort\nUser timezone: America\/Los_Angeles/);
  });

  it('always appends current time section', async () => {
    const prompt = await (client as any).buildSystemPrompt({
      messages: [{ role: 'user', content: 'hello' }],
      injectBehaviors: false,
    });
    expect(prompt).toMatch(/## Current Time\n/);
  });

  it('injects active goals when agentId is set and goals provider is wired', async () => {
    (client as any).setGoals({
      listForAgent: (_agentId: string) => [
        { id: 'g1', title: 'Ship Fort v1', status: 'active' },
        { id: 'g2', title: 'Hire two engineers', status: 'active' },
      ],
    });
    const prompt = await (client as any).buildSystemPrompt({
      messages: [{ role: 'user', content: 'hello' }],
      agentId: 'agent-1',
      injectBehaviors: false,
    });
    expect(prompt).toMatch(/## Active Goals\n/);
    expect(prompt).toMatch(/- Ship Fort v1/);
    expect(prompt).toMatch(/- Hire two engineers/);
  });

  it('does NOT inject goals when system prompt is overridden (Triager/Hatch path)', async () => {
    (client as any).setGoals({
      listForAgent: () => [{ id: 'g1', title: 'Ship Fort v1', status: 'active' }],
    });
    const prompt = await (client as any).buildSystemPrompt({
      messages: [{ role: 'user', content: 'classify this' }],
      system: 'You are the Triager.',
      agentId: 'agent-1',
      injectBehaviors: false,
    });
    expect(prompt).not.toMatch(/## Active Goals/);
    expect(prompt).not.toMatch(/Ship Fort v1/);
  });

  it('injects profile facts under "## About the User" when present in memory', () => {
    memory.createNode({ type: 'profile', label: 'Lives in Wellington, NZ', source: 'hatch' });
    memory.createNode({ type: 'profile', label: 'Prefers terse responses for quick lookups', source: 'hatch' });
    return (client as any).buildSystemPrompt({
      messages: [{ role: 'user', content: 'hello' }],
      agentId: 'agent-1',
      injectBehaviors: false,
    }).then((prompt: string) => {
      expect(prompt).toMatch(/## About the User\n/);
      expect(prompt).toMatch(/- Lives in Wellington, NZ/);
      expect(prompt).toMatch(/- Prefers terse responses/);
    });
  });

  it('injectAgentContext:false suppresses goals + profile even when agentId is set', async () => {
    (client as any).setGoals({
      listForAgent: () => [{ id: 'g1', title: 'Ship Fort v1', status: 'active' }],
    });
    memory.createNode({ type: 'profile', label: 'Lives in Wellington', source: 'hatch' });
    const prompt = await (client as any).buildSystemPrompt({
      messages: [{ role: 'user', content: 'hello' }],
      agentId: 'agent-1',
      injectAgentContext: false,
      injectBehaviors: false,
    });
    expect(prompt).not.toMatch(/## Active Goals/);
    expect(prompt).not.toMatch(/## About the User/);
  });
});
