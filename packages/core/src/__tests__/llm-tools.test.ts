/**
 * LLM Tools Tests — Phase 2: Claude tool_use wiring
 *
 * Tests that LLMClient correctly:
 *   - Converts FortTool[] to Claude's tools format
 *   - Handles tool_use responses and loops via completeWithTools()
 *   - Returns final text after tool_result
 *   - Stops at max iterations
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { mkdtempSync, rmSync } from 'node:fs';
import { LLMClient } from '../llm/index.js';
import type { LLMToolsResponse } from '../llm/index.js';
import { ToolExecutor } from '../tools/executor.js';
import type { FortTool, ToolResult } from '../tools/types.js';
import { PermissionManager } from '../permissions/index.js';
import { ModuleBus } from '../module-bus/index.js';
import { TokenTracker } from '../tokens/index.js';

// ─── Helpers ────────────────────────────────────────────────────────

function makeTool(overrides: Partial<FortTool> & Pick<FortTool, 'tier'>): FortTool {
  return {
    name: 'test-tool',
    description: 'A test tool that does things',
    inputSchema: {
      type: 'object',
      properties: { query: { type: 'string' } },
      required: ['query'],
    },
    execute: async (_input: unknown): Promise<ToolResult> => ({
      success: true,
      output: 'tool result',
    }),
    ...overrides,
  };
}

/** Build a fake Anthropic tool_use response */
function makeToolUseResponse(toolName: string, toolId: string, input: object) {
  return {
    id: 'msg_001',
    type: 'message',
    role: 'assistant',
    content: [{ type: 'tool_use', id: toolId, name: toolName, input }],
    stop_reason: 'tool_use',
    stop_sequence: null,
    usage: { input_tokens: 50, output_tokens: 20 },
    model: 'claude-haiku-4-5-20251001',
  };
}

/** Build a fake Anthropic text response */
function makeTextResponse(text: string) {
  return {
    id: 'msg_002',
    type: 'message',
    role: 'assistant',
    content: [{ type: 'text', text }],
    stop_reason: 'end_turn',
    stop_sequence: null,
    usage: { input_tokens: 80, output_tokens: 30 },
    model: 'claude-haiku-4-5-20251001',
  };
}

// ─── Suite ──────────────────────────────────────────────────────────

describe('LLMClient — tool_use wiring', () => {
  let tmpDir: string;
  let bus: ModuleBus;
  let tokens: TokenTracker;
  let permissions: PermissionManager;
  let executor: ToolExecutor;
  let llm: LLMClient;
  let mockCreate: ReturnType<typeof vi.fn>;
  let savedOAuthToken: string | undefined;
  let savedApiKey: string | undefined;

  beforeEach(() => {
    // Isolate from ambient auth (same pattern as llm.test.ts)
    savedOAuthToken = process.env.CLAUDE_CODE_OAUTH_TOKEN;
    savedApiKey = process.env.ANTHROPIC_API_KEY;
    delete process.env.CLAUDE_CODE_OAUTH_TOKEN;
    delete process.env.ANTHROPIC_API_KEY;

    tmpDir = mkdtempSync(join(tmpdir(), 'fort-llm-tools-'));
    bus = new ModuleBus();
    tokens = new TokenTracker(join(tmpDir, 'tokens.db'), bus);
    permissions = new PermissionManager(join(tmpDir, 'permissions.yaml'));
    executor = new ToolExecutor(permissions, bus, tokens);

    vi.spyOn(LLMClient, 'readEnvFile').mockReturnValue(null);
    vi.spyOn(LLMClient, 'readKeychainToken').mockReturnValue(null);

    llm = new LLMClient({ apiKey: 'test-key' }, bus, tokens);

    // Inject mock Anthropic client
    mockCreate = vi.fn();
    (llm as any).client = { messages: { create: mockCreate } };
  });

  afterEach(() => {
    vi.restoreAllMocks();
    if (savedOAuthToken !== undefined) process.env.CLAUDE_CODE_OAUTH_TOKEN = savedOAuthToken;
    else delete process.env.CLAUDE_CODE_OAUTH_TOKEN;
    if (savedApiKey !== undefined) process.env.ANTHROPIC_API_KEY = savedApiKey;
    else delete process.env.ANTHROPIC_API_KEY;
    tokens.close();
    rmSync(tmpDir, { recursive: true, force: true });
  });

  // ── complete() with tools ───────────────────────────────────────────

  describe('complete() with tools parameter', () => {
    it('passes tools to Claude in the correct format', async () => {
      mockCreate.mockResolvedValue(makeTextResponse('hello'));

      const tool = makeTool({ tier: 1, name: 'my-tool', description: 'Does something' });
      await llm.complete({
        messages: [{ role: 'user', content: 'hi' }],
        tools: [tool],
      });

      const callArgs = mockCreate.mock.calls[0][0];
      expect(callArgs.tools).toBeDefined();
      expect(callArgs.tools).toHaveLength(1);
      expect(callArgs.tools[0].name).toBe('my-tool');
      expect(callArgs.tools[0].description).toBe('Does something');
      expect(callArgs.tools[0].input_schema).toBeDefined();
      expect(callArgs.tools[0].input_schema.type).toBe('object');
    });

    it('converts inputSchema → input_schema (snake_case)', async () => {
      mockCreate.mockResolvedValue(makeTextResponse('ok'));

      const tool = makeTool({
        tier: 1,
        name: 'schema-tool',
        inputSchema: {
          type: 'object',
          properties: { path: { type: 'string' } },
          required: ['path'],
        },
      });

      await llm.complete({
        messages: [{ role: 'user', content: 'test' }],
        tools: [tool],
      });

      const { tools } = mockCreate.mock.calls[0][0];
      expect(tools[0].input_schema.properties.path).toEqual({ type: 'string' });
      expect(tools[0].input_schema.required).toEqual(['path']);
    });

    it('does not send tools param when tools array is empty', async () => {
      mockCreate.mockResolvedValue(makeTextResponse('hi'));

      await llm.complete({
        messages: [{ role: 'user', content: 'hi' }],
        tools: [],
      });

      const callArgs = mockCreate.mock.calls[0][0];
      expect(callArgs.tools).toBeUndefined();
    });

    it('does not send tools param when tools is not provided', async () => {
      mockCreate.mockResolvedValue(makeTextResponse('hi'));

      await llm.complete({
        messages: [{ role: 'user', content: 'hi' }],
      });

      const callArgs = mockCreate.mock.calls[0][0];
      expect(callArgs.tools).toBeUndefined();
    });

    it('returns text content when stop_reason is end_turn (tool not used)', async () => {
      mockCreate.mockResolvedValue(makeTextResponse('No tools needed'));

      const tool = makeTool({ tier: 1 });
      const result = await llm.complete({
        messages: [{ role: 'user', content: 'hi' }],
        tools: [tool],
      });

      expect(result.content).toBe('No tools needed');
      expect(result.stopReason).toBe('end_turn');
    });
  });

  // ── completeWithTools() ─────────────────────────────────────────────

  describe('completeWithTools()', () => {
    it('throws when client is not configured', async () => {
      vi.spyOn(LLMClient, 'readEnvFile').mockReturnValue(null);
      const unconfigured = new LLMClient({}, bus, tokens);
      const tool = makeTool({ tier: 1 });

      await expect(
        unconfigured.completeWithTools(
          { messages: [{ role: 'user', content: 'hi' }], tools: [tool] },
          executor,
        ),
      ).rejects.toThrow('LLM client not configured');
    });

    it('returns text immediately when first response has no tool_use', async () => {
      mockCreate.mockResolvedValue(makeTextResponse('Done, no tools needed.'));

      const tool = makeTool({ tier: 1, name: 'unused-tool' });
      const result: LLMToolsResponse = await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'say hi' }], tools: [tool] },
        executor,
      );

      expect(result.content).toBe('Done, no tools needed.');
      expect(result.iterations).toBe(1);
      expect(result.toolCallLog).toHaveLength(0);
      expect(mockCreate).toHaveBeenCalledTimes(1);
    });

    it('sends tools in Claude format during multi-turn loop', async () => {
      mockCreate
        .mockResolvedValueOnce(makeToolUseResponse('my-tool', 'call_1', { query: 'test' }))
        .mockResolvedValueOnce(makeTextResponse('Final answer'));

      const tool = makeTool({ tier: 1, name: 'my-tool' });
      await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'use the tool' }], tools: [tool] },
        executor,
      );

      // Both calls should include tools
      for (const call of mockCreate.mock.calls) {
        expect(call[0].tools).toBeDefined();
        expect(call[0].tools[0].name).toBe('my-tool');
      }
    });

    it('executes tool and appends tool_result before re-sending', async () => {
      mockCreate
        .mockResolvedValueOnce(makeToolUseResponse('my-tool', 'call_abc', { query: 'hello' }))
        .mockResolvedValueOnce(makeTextResponse('Got the result'));

      const tool = makeTool({
        tier: 1,
        name: 'my-tool',
        execute: async () => ({ success: true, output: 'tool output here' }),
      });

      const result = await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'go' }], tools: [tool] },
        executor,
      );

      expect(result.content).toBe('Got the result');
      expect(result.iterations).toBe(2);

      // Second API call should include tool_result in messages
      const secondCallMessages = mockCreate.mock.calls[1][0].messages;
      // [0] = original user msg, [1] = assistant tool_use, [2] = user tool_result
      expect(secondCallMessages).toHaveLength(3);
      const toolResultMsg = secondCallMessages[2];
      expect(toolResultMsg.role).toBe('user');
      expect(toolResultMsg.content[0].type).toBe('tool_result');
      expect(toolResultMsg.content[0].tool_use_id).toBe('call_abc');
      expect(toolResultMsg.content[0].content).toBe('tool output here');
      expect(toolResultMsg.content[0].is_error).toBe(false);
    });

    it('logs successful tool call in toolCallLog', async () => {
      mockCreate
        .mockResolvedValueOnce(makeToolUseResponse('my-tool', 'call_log1', { query: 'x' }))
        .mockResolvedValueOnce(makeTextResponse('done'));

      const tool = makeTool({ tier: 1, name: 'my-tool' });
      const result = await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'go' }], tools: [tool] },
        executor,
      );

      expect(result.toolCallLog).toHaveLength(1);
      expect(result.toolCallLog[0].toolName).toBe('my-tool');
      expect(result.toolCallLog[0].denied).toBe(false);
    });

    it('logs denied tool call when tier 4 tool is invoked', async () => {
      mockCreate
        .mockResolvedValueOnce(makeToolUseResponse('forbidden-tool', 'call_denied', {}))
        .mockResolvedValueOnce(makeTextResponse('ok, blocked'));

      const tool = makeTool({ tier: 4, name: 'forbidden-tool' });
      const result = await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'try it' }], tools: [tool] },
        executor,
      );

      expect(result.toolCallLog).toHaveLength(1);
      expect(result.toolCallLog[0].toolName).toBe('forbidden-tool');
      expect(result.toolCallLog[0].denied).toBe(true);
    });

    it('marks tool_result as is_error when tool execution fails', async () => {
      mockCreate
        .mockResolvedValueOnce(makeToolUseResponse('bad-tool', 'call_err', {}))
        .mockResolvedValueOnce(makeTextResponse('noted the error'));

      const tool = makeTool({
        tier: 1,
        name: 'bad-tool',
        execute: async () => ({ success: false, output: '', error: 'something broke' }),
      });

      await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'go' }], tools: [tool] },
        executor,
      );

      const secondCallMessages = mockCreate.mock.calls[1][0].messages;
      const toolResult = secondCallMessages[2].content[0];
      expect(toolResult.is_error).toBe(true);
      expect(toolResult.content).toBe('something broke');
    });

    it('handles unknown tool name gracefully', async () => {
      mockCreate
        .mockResolvedValueOnce(makeToolUseResponse('ghost-tool', 'call_ghost', {}))
        .mockResolvedValueOnce(makeTextResponse('could not find it'));

      // No tool named 'ghost-tool' in the tool list
      const tool = makeTool({ tier: 1, name: 'other-tool' });
      const result = await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'go' }], tools: [tool] },
        executor,
      );

      expect(result.content).toBe('could not find it');

      const secondCallMessages = mockCreate.mock.calls[1][0].messages;
      const toolResult = secondCallMessages[2].content[0];
      expect(toolResult.is_error).toBe(true);
      expect(toolResult.content).toContain('ghost-tool');
      expect(toolResult.content).toContain('not found');
    });

    it('handles multiple tool calls in a single response', async () => {
      // Simulate Claude requesting two tools at once
      const multiToolResponse = {
        ...makeToolUseResponse('tool-a', 'call_a', { query: 'foo' }),
        content: [
          { type: 'tool_use', id: 'call_a', name: 'tool-a', input: { query: 'foo' } },
          { type: 'tool_use', id: 'call_b', name: 'tool-b', input: { query: 'bar' } },
        ],
      };
      mockCreate
        .mockResolvedValueOnce(multiToolResponse)
        .mockResolvedValueOnce(makeTextResponse('both done'));

      const toolA = makeTool({
        tier: 1,
        name: 'tool-a',
        execute: async () => ({ success: true, output: 'result-a' }),
      });
      const toolB = makeTool({
        tier: 1,
        name: 'tool-b',
        execute: async () => ({ success: true, output: 'result-b' }),
      });

      const result = await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'use both' }], tools: [toolA, toolB] },
        executor,
      );

      expect(result.toolCallLog).toHaveLength(2);
      expect(result.toolCallLog.map((l) => l.toolName).sort()).toEqual(['tool-a', 'tool-b']);

      // Both tool results sent back in same user message
      const secondCallMessages = mockCreate.mock.calls[1][0].messages;
      const toolResultMsg = secondCallMessages[2];
      expect(toolResultMsg.content).toHaveLength(2);
      expect(toolResultMsg.content[0].tool_use_id).toBe('call_a');
      expect(toolResultMsg.content[1].tool_use_id).toBe('call_b');
    });

    it('stops at max iterations and returns empty content', async () => {
      // Always return tool_use — should hit the cap
      mockCreate.mockResolvedValue(
        makeToolUseResponse('loop-tool', 'call_loop', { query: 'x' }),
      );

      const tool = makeTool({ tier: 1, name: 'loop-tool' });
      const result = await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'loop' }], tools: [tool] },
        executor,
        { maxIterations: 3 },
      );

      expect(result.iterations).toBe(3);
      expect(result.content).toBe('');
      expect(result.stopReason).toBe('max_iterations');
      expect(mockCreate).toHaveBeenCalledTimes(3);
    });

    it('defaults to max 10 iterations', async () => {
      mockCreate.mockResolvedValue(
        makeToolUseResponse('loop-tool', 'call_inf', { query: 'y' }),
      );

      const tool = makeTool({ tier: 1, name: 'loop-tool' });
      const result = await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'loop forever' }], tools: [tool] },
        executor,
      );

      expect(result.iterations).toBe(10);
      expect(mockCreate).toHaveBeenCalledTimes(10);
    });

    it('aggregates token counts across all iterations', async () => {
      mockCreate
        .mockResolvedValueOnce(makeToolUseResponse('my-tool', 'c1', {}))
        .mockResolvedValueOnce(makeTextResponse('done'));

      // First response: 50 in + 20 out, Second: 80 in + 30 out
      const tool = makeTool({ tier: 1, name: 'my-tool' });
      const result = await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'go' }], tools: [tool] },
        executor,
      );

      expect(result.inputTokens).toBe(50 + 80);
      expect(result.outputTokens).toBe(20 + 30);
      expect(result.totalTokens).toBe(result.inputTokens + result.outputTokens);
    });

    it('publishes llm.completed bus event on success', async () => {
      const completedEvents: unknown[] = [];
      bus.subscribe('llm.completed', (e) => { completedEvents.push(e); });

      mockCreate.mockResolvedValue(makeTextResponse('ok'));
      const tool = makeTool({ tier: 1, name: 'unused' });

      await llm.completeWithTools(
        { messages: [{ role: 'user', content: 'hi' }], tools: [tool] },
        executor,
      );

      expect(completedEvents).toHaveLength(1);
    });
  });
});
