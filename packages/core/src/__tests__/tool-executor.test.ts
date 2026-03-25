import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { ToolExecutor } from '../tools/executor.js';
import type { FortTool, ToolResult } from '../tools/types.js';
import { PermissionManager } from '../permissions/index.js';
import { ModuleBus } from '../module-bus/index.js';
import { TokenTracker } from '../tokens/index.js';
import type { FortEvent } from '../types.js';

// ─── Helpers ────────────────────────────────────────────────────────

function makeTool(overrides: Partial<FortTool> & { tier: FortTool['tier'] }): FortTool {
  return {
    name: 'test-tool',
    description: 'A test tool',
    inputSchema: { type: 'object' },
    execute: async (_input: unknown): Promise<ToolResult> => ({
      success: true,
      output: 'ok',
    }),
    ...overrides,
  };
}

// ─── Suite ──────────────────────────────────────────────────────────

describe('ToolExecutor', () => {
  let tmpDir: string;
  let permissions: PermissionManager;
  let bus: ModuleBus;
  let tokens: TokenTracker;
  let executor: ToolExecutor;

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), 'fort-executor-'));
    permissions = new PermissionManager(join(tmpDir, 'permissions.yaml'));
    bus = new ModuleBus();
    tokens = new TokenTracker(join(tmpDir, 'tokens.db'), bus);
    executor = new ToolExecutor(permissions, bus, tokens);
  });

  afterEach(() => {
    tokens.close();
    if (tmpDir) rmSync(tmpDir, { recursive: true, force: true });
  });

  // ── Tier 1 (auto) ──────────────────────────────────────────────────

  it('executes a tier 1 tool without prompting', async () => {
    const tool = makeTool({ tier: 1, name: 'tier1-tool' });
    const result = await executor.execute(tool, { query: 'hello' });

    expect(result.success).toBe(true);
    expect(result.output).toBe('ok');
    expect(result.error).toBeUndefined();
  });

  it('publishes tool.executed event for a tier 1 tool', async () => {
    const events: FortEvent[] = [];
    bus.subscribe('tool.executed', (e) => { events.push(e); });

    const tool = makeTool({ tier: 1, name: 'tier1-event-tool' });
    await executor.execute(tool, 'input');

    expect(events).toHaveLength(1);
    expect(events[0].type).toBe('tool.executed');
    const log = events[0].payload as any;
    expect(log.toolName).toBe('tier1-event-tool');
    expect(log.denied).toBe(false);
  });

  // ── Tier 2 (draft) ─────────────────────────────────────────────────

  it('executes a tier 2 tool (creates draft) without blocking', async () => {
    const tool = makeTool({ tier: 2, name: 'tier2-tool' });
    const result = await executor.execute(tool, {});

    expect(result.success).toBe(true);
    expect(result.output).toBe('ok');
  });

  it('publishes tool.executed for a tier 2 tool', async () => {
    const events: FortEvent[] = [];
    bus.subscribe('tool.executed', (e) => { events.push(e); });

    const tool = makeTool({ tier: 2, name: 'tier2-event-tool' });
    await executor.execute(tool, {});

    expect(events).toHaveLength(1);
    expect(events[0].payload as any).toMatchObject({ toolName: 'tier2-event-tool', denied: false });
  });

  // ── Tier 3 (approve) ───────────────────────────────────────────────

  it('blocks a tier 3 tool and returns denial result', async () => {
    const tool = makeTool({ tier: 3, name: 'tier3-tool' });
    const result = await executor.execute(tool, {});

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/tier 3/i);
    expect(result.error).toMatch(/approval/i);
  });

  it('publishes tool.denied event for a tier 3 tool', async () => {
    const deniedEvents: FortEvent[] = [];
    const executedEvents: FortEvent[] = [];
    bus.subscribe('tool.denied', (e) => { deniedEvents.push(e); });
    bus.subscribe('tool.executed', (e) => { executedEvents.push(e); });

    const tool = makeTool({ tier: 3, name: 'tier3-blocked' });
    await executor.execute(tool, {});

    expect(deniedEvents).toHaveLength(1);
    expect(executedEvents).toHaveLength(0);
    const log = deniedEvents[0].payload as any;
    expect(log.denied).toBe(true);
    expect(log.denialReason).toMatch(/tier 3/i);
  });

  // ── Tier 4 (never) ─────────────────────────────────────────────────

  it('blocks a tier 4 tool and returns denial result', async () => {
    const tool = makeTool({ tier: 4, name: 'tier4-tool' });
    const result = await executor.execute(tool, {});

    expect(result.success).toBe(false);
    expect(result.error).toMatch(/tier 4/i);
  });

  it('publishes tool.denied event for a tier 4 tool', async () => {
    const events: FortEvent[] = [];
    bus.subscribe('tool.denied', (e) => { events.push(e); });

    const tool = makeTool({ tier: 4, name: 'tier4-blocked' });
    await executor.execute(tool, {});

    expect(events).toHaveLength(1);
    const log = events[0].payload as any;
    expect(log.toolName).toBe('tier4-blocked');
    expect(log.denied).toBe(true);
    expect(log.denialReason).toMatch(/tier 4/i);
  });

  // ── Error handling ──────────────────────────────────────────────────

  it('publishes tool.error when execute() throws', async () => {
    const errorEvents: FortEvent[] = [];
    bus.subscribe('tool.error', (e) => { errorEvents.push(e); });

    const tool = makeTool({
      tier: 1,
      name: 'failing-tool',
      execute: async () => { throw new Error('something went wrong'); },
    });

    const result = await executor.execute(tool, {});

    expect(result.success).toBe(false);
    expect(result.error).toBe('something went wrong');
    expect(errorEvents).toHaveLength(1);
    const log = errorEvents[0].payload as any;
    expect(log.toolName).toBe('failing-tool');
    expect(log.denied).toBe(false);
  });

  it('returns a ToolResult even when the tool throws a non-Error', async () => {
    const tool = makeTool({
      tier: 1,
      name: 'string-throw-tool',
      execute: async () => { throw 'plain string error'; },
    });

    const result = await executor.execute(tool, {});
    expect(result.success).toBe(false);
    expect(result.error).toBe('plain string error');
  });

  // ── Token tracking ──────────────────────────────────────────────────

  it('records a token entry when a tier 1 tool succeeds', async () => {
    const tool = makeTool({ tier: 1, name: 'tracked-tool' });
    const before = tokens.getStats().allTime.calls;

    await executor.execute(tool, {});

    const after = tokens.getStats().allTime.calls;
    expect(after).toBe(before + 1);
  });

  it('does not record a token entry when the tool is denied', async () => {
    const tool = makeTool({ tier: 4, name: 'denied-tracked' });
    const before = tokens.getStats().allTime.calls;

    await executor.execute(tool, {});

    // No token record for denied calls
    const after = tokens.getStats().allTime.calls;
    expect(after).toBe(before);
  });

  // ── Log payload shape ───────────────────────────────────────────────

  it('includes taskId and agentId in the published log', async () => {
    const events: FortEvent[] = [];
    bus.subscribe('tool.executed', (e) => { events.push(e); });

    const tool = makeTool({ tier: 1, name: 'ctx-tool' });
    await executor.execute(tool, {}, { taskId: 'task-123', agentId: 'agent-456' });

    const log = events[0].payload as any;
    expect(log.taskId).toBe('task-123');
    expect(log.agentId).toBe('agent-456');
  });

  it('includes durationMs in the published log', async () => {
    const events: FortEvent[] = [];
    bus.subscribe('tool.executed', (e) => { events.push(e); });

    const tool = makeTool({ tier: 1, name: 'timed-tool' });
    await executor.execute(tool, {});

    const log = events[0].payload as any;
    expect(typeof log.durationMs).toBe('number');
    expect(log.durationMs).toBeGreaterThanOrEqual(0);
  });

  it('includes input and result in the log', async () => {
    const events: FortEvent[] = [];
    bus.subscribe('tool.executed', (e) => { events.push(e); });

    const input = { query: 'test input' };
    const tool = makeTool({ tier: 1, name: 'input-output-tool' });
    await executor.execute(tool, input);

    const log = events[0].payload as any;
    expect(log.input).toEqual(input);
    expect(log.result.success).toBe(true);
    expect(log.result.output).toBe('ok');
  });

  // ── ToolRegistry integration ────────────────────────────────────────

  it('registerTool stores a live callable and retrieves it', async () => {
    const { ToolRegistry } = await import('../tools/index.js');
    const registry = new ToolRegistry(join(tmpDir, 'tools.db'));

    const tool = makeTool({ tier: 1, name: 'live-tool' });
    const definition = registry.registerTool(tool);

    expect(definition.name).toBe('live-tool');
    expect(definition.id).toBeTruthy();

    const retrieved = registry.getLiveTool('live-tool');
    expect(retrieved).not.toBeNull();
    expect(retrieved!.name).toBe('live-tool');

    // Execute via the live tool reference
    const result = await retrieved!.execute({});
    expect(result.success).toBe(true);

    registry.close();
  });

  it('listLiveTools returns all registered tools', async () => {
    const { ToolRegistry } = await import('../tools/index.js');
    const registry = new ToolRegistry(join(tmpDir, 'tools2.db'));

    registry.registerTool(makeTool({ tier: 1, name: 'tool-a' }));
    registry.registerTool(makeTool({ tier: 2, name: 'tool-b' }));

    const live = registry.listLiveTools();
    expect(live).toHaveLength(2);
    expect(live.map((t) => t.name).sort()).toEqual(['tool-a', 'tool-b']);

    registry.close();
  });

  it('getLiveTool returns null for metadata-only registrations', async () => {
    const { ToolRegistry } = await import('../tools/index.js');
    const registry = new ToolRegistry(join(tmpDir, 'tools3.db'));

    registry.register({
      name: 'metadata-only',
      description: 'No execute()',
      capabilities: [],
      inputTypes: [],
      outputTypes: [],
      tags: [],
      module: 'test',
      version: '1.0.0',
    });

    expect(registry.getLiveTool('metadata-only')).toBeNull();

    registry.close();
  });
});
