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
 *   approval.required — Tier 2/3 tool awaiting user approval
 */

import { v4 as uuid } from 'uuid';
import type { PermissionManager } from '../permissions/index.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { TokenTracker } from '../tokens/index.js';
import type { FortTool, ToolResult, ToolCallLog } from './types.js';
import type { ApprovalStore } from './approval-store.js';

export interface ToolExecutorOptions {
  /** Task context forwarded to TokenTracker records */
  taskId?: string;
  /** Agent invoking the tool */
  agentId?: string;
}

/** Thrown when a user explicitly rejects a tool request */
export class ToolRejectedError extends Error {
  constructor(toolName: string, reason?: string) {
    super(`Tool "${toolName}" was rejected by user${reason ? `: ${reason}` : ''}`);
    this.name = 'ToolRejectedError';
  }
}

export class ToolExecutor {
  private permissions: PermissionManager;
  private bus: ModuleBus;
  private tokens: TokenTracker;
  private approvalStore: ApprovalStore | null = null;

  /** Pending approval promises keyed by approvalId */
  private pendingApprovals: Map<string, {
    resolve: (result: { approved: boolean; reason?: string }) => void;
  }> = new Map();

  constructor(permissions: PermissionManager, bus: ModuleBus, tokens: TokenTracker) {
    this.permissions = permissions;
    this.bus = bus;
    this.tokens = tokens;
  }

  /**
   * Attach an ApprovalStore so Tier 2/3 tools block for user approval.
   * When not set, Tier 3 tools are denied immediately (legacy behaviour).
   */
  setApprovalStore(store: ApprovalStore): void {
    this.approvalStore = store;
  }

  /**
   * Resolve a pending approval from outside (called by WS handler).
   * Publishes 'approval.resolved' on the bus so awaitApproval() unblocks.
   */
  resolveApproval(id: string, approved: boolean, rejectionReason?: string): void {
    const pending = this.pendingApprovals.get(id);
    if (pending) {
      pending.resolve({ approved, reason: rejectionReason });
    }
  }

  /**
   * Execute a tool after checking permissions.
   *
   * - Tier 1 (auto): executes immediately
   * - Tier 2 (draft): blocks until user approves (requires ApprovalStore)
   * - Tier 3 (approve): blocks until user approves (requires ApprovalStore)
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

    // ── Tier 2/3: require user approval ──────────────────────────────
    if (tool.tier === 3 || tool.tier === 2) {
      if (!this.approvalStore) {
        // No approval store — fall back to legacy deny for tier 3, pass-through for tier 2
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
        // Tier 2 without approval store: auto-approve (dev mode)
      } else {
        // Block until approved or timed out
        try {
          await this.awaitApproval(tool, input, options, logId, startedAt, calledAt);
        } catch (err) {
          // awaitApproval throws ToolRejectedError or timeout Error — surface as ToolResult
          const errorMessage = err instanceof Error ? err.message : String(err);
          const result: ToolResult = {
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
            denied: true,
            denialReason: errorMessage,
            taskId: options.taskId,
            agentId: options.agentId,
            calledAt,
          };

          await this.bus.publish('tool.denied', 'tool-primers', log);
          return result;
        }
      }
    }

    // ── Execute ──────────────────────────────────────────────────────
    let result: ToolResult;
    try {
      result = await tool.execute(input, { taskId: options.taskId, agentId: options.agentId });
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

  /**
   * Internal: create approval record, publish event, block until resolved or timed out.
   * Resolves silently on approval; throws on rejection or timeout.
   */
  private async awaitApproval(
    tool: FortTool,
    input: unknown,
    options: ToolExecutorOptions,
    _logId: string,
    _startedAt: number,
    _calledAt: Date,
  ): Promise<void> {
    const approval = this.approvalStore!.create({
      taskId: options.taskId ?? 'unknown',
      agentId: options.agentId ?? 'unknown',
      toolName: tool.name,
      toolTier: tool.tier,
      parameters: input,
    });

    await this.bus.publish('approval.required', 'tool-executor', {
      approvalId: approval.id,
      toolName: tool.name,
      tier: tool.tier,
      taskId: options.taskId,
      agentId: options.agentId,
      parameters: input,
    });

    return new Promise<void>((resolve, reject) => {
      const timeoutMs = 600_000; // 10 minutes

      const timer = setTimeout(() => {
        this.pendingApprovals.delete(approval.id);
        reject(new Error(`Approval timed out for tool "${tool.name}" (10 min)`));
      }, timeoutMs);

      this.pendingApprovals.set(approval.id, {
        resolve: (result) => {
          clearTimeout(timer);
          this.pendingApprovals.delete(approval.id);
          if (result.approved) {
            resolve();
          } else {
            reject(new ToolRejectedError(tool.name, result.reason));
          }
        },
      });
    });
  }
}
