/**
 * Tool Executor — Checked, Tracked, Observable Tool Execution
 *
 * ToolExecutor is the single point of truth for running a FortTool.
 * It gates execution through PermissionManager, records usage in
 * TokenTracker, and publishes observable events on ModuleBus.
 *
 * Events published:
 *   tool.executed  — successful execution (includes result)
 *   tool.denied    — permission denied before execution
 *   tool.error     — exception thrown during execution
 */

import { v4 as uuid } from 'uuid';
import type { PermissionManager } from '../permissions/index.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TokenTracker } from '../tokens/index.js';
import type { FortTool, ToolResult, ToolCallLog } from './types.js';

export interface ToolExecutorOptions {
  /** Task context forwarded to TokenTracker records */
  taskId?: string;
  /** Agent invoking the tool */
  agentId?: string;
}

export class ToolExecutor {
  private permissions: PermissionManager;
  private bus: ModuleBus;
  private tokens: TokenTracker;

  constructor(permissions: PermissionManager, bus: ModuleBus, tokens: TokenTracker) {
    this.permissions = permissions;
    this.bus = bus;
    this.tokens = tokens;
  }

  /**
   * Execute a tool after checking permissions.
   *
   * - Tier 1 (auto): executes immediately
   * - Tier 2 (draft): executes immediately (caller is responsible for draft UX)
   * - Tier 3 (approve): denied — caller must obtain approval before calling
   * - Tier 4 (never): always denied
   *
   * Publishes tool.executed, tool.denied, or tool.error events.
   * Records execution cost in TokenTracker.
   * Returns ToolResult in all cases (success or denial).
   */
  async execute(
    tool: FortTool,
    input: unknown,
    options: ToolExecutorOptions = {},
  ): Promise<ToolResult> {
    const startedAt = Date.now();
    const calledAt = new Date();
    const logId = uuid();

    // ── Permission check ─────────────────────────────────────────────
    if (tool.tier === 4) {
      const result: ToolResult = {
        success: false,
        output: '',
        error: `Tool "${tool.name}" is tier 4 (never) and cannot be executed.`,
      };

      const log: ToolCallLog = {
        id: logId,
        toolName: tool.name,
        input,
        result,
        durationMs: Date.now() - startedAt,
        denied: true,
        denialReason: 'Tier 4 (never) — execution permanently blocked',
        taskId: options.taskId,
        agentId: options.agentId,
        calledAt,
      };

      await this.bus.publish('tool.denied', 'tool-executor', log);
      return result;
    }

    if (tool.tier === 3) {
      const result: ToolResult = {
        success: false,
        output: '',
        error: `Tool "${tool.name}" is tier 3 (approve) and requires explicit approval before execution.`,
      };

      const log: ToolCallLog = {
        id: logId,
        toolName: tool.name,
        input,
        result,
        durationMs: Date.now() - startedAt,
        denied: true,
        denialReason: 'Tier 3 (approve) — awaiting explicit user approval',
        taskId: options.taskId,
        agentId: options.agentId,
        calledAt,
      };

      await this.bus.publish('tool.denied', 'tool-executor', log);
      return result;
    }

    // ── Execute ──────────────────────────────────────────────────────
    let result: ToolResult;
    try {
      result = await tool.execute(input);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : String(err);
      result = {
        success: false,
        output: '',
        error: errorMessage,
      };

      const log: ToolCallLog = {
        id: logId,
        toolName: tool.name,
        input,
        result,
        durationMs: Date.now() - startedAt,
        denied: false,
        taskId: options.taskId,
        agentId: options.agentId,
        calledAt,
      };

      await this.bus.publish('tool.error', 'tool-executor', log);
      return result;
    }

    const durationMs = Date.now() - startedAt;

    const log: ToolCallLog = {
      id: logId,
      toolName: tool.name,
      input,
      result,
      durationMs,
      denied: false,
      taskId: options.taskId,
      agentId: options.agentId,
      calledAt,
    };

    // ── Record in TokenTracker ───────────────────────────────────────
    // Tool calls are tracked as zero-token entries to represent cost/activity
    await this.tokens.record({
      timestamp: calledAt,
      model: `tool:${tool.name}`,
      inputTokens: 0,
      outputTokens: 0,
      totalTokens: 0,
      costUsd: 0,
      taskId: options.taskId,
      agentId: options.agentId,
      source: 'tool-executor',
    });

    await this.bus.publish('tool.executed', 'tool-executor', log);
    return result;
  }
}
