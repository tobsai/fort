/**
 * delegate-to-agent — Agent Handoff Tool (Tier 1)
 *
 * Allows one agent to delegate a subtask to another specialist agent.
 * Creates a child task, dispatches it to the target agent, and waits
 * for the result before returning. Detects cycles and enforces max depth.
 */

import type { TaskGraph } from '../task-graph/index.js';
import type { ModuleBus } from '../module-bus/index.js';
import type { AgentRegistry } from '../agents/index.js';
import type { FortTool, ToolResult, ToolExecutionContext } from './types.js';
import type { Task } from '../types.js';

const MAX_DELEGATION_DEPTH = 5;
const SUBTASK_TIMEOUT_MS = 10 * 60 * 1000; // 10 minutes

interface DelegateParams {
  agentId: string;
  taskTitle: string;
  taskDescription: string;
  expectation: string;
}

function isDelegateParams(v: unknown): v is DelegateParams {
  if (!v || typeof v !== 'object') return false;
  const o = v as Record<string, unknown>;
  return (
    typeof o['agentId'] === 'string' &&
    typeof o['taskTitle'] === 'string' &&
    typeof o['taskDescription'] === 'string' &&
    typeof o['expectation'] === 'string'
  );
}

/**
 * Walk up the parent chain and collect all (taskId, assignedAgent) pairs.
 * Returns the chain as an array of Tasks from the current task up to the root.
 */
function buildParentChain(taskGraph: TaskGraph, startTaskId: string): Task[] {
  const chain: Task[] = [];
  let current: Task | null = null;
  try {
    current = taskGraph.getTask(startTaskId);
  } catch {
    return chain;
  }
  chain.push(current);

  while (current.parentId) {
    try {
      current = taskGraph.getTask(current.parentId);
      chain.push(current);
    } catch {
      break;
    }
  }
  return chain;
}

export function createDelegateTool(
  taskGraph: TaskGraph,
  bus: ModuleBus,
  agents: AgentRegistry,
): FortTool {
  return {
    name: 'delegate-to-agent',
    description: 'Delegate a subtask to a specialist agent and wait for the result. Use this when a task requires capabilities of a different agent.',
    tier: 1,
    inputSchema: {
      type: 'object',
      properties: {
        agentId: { type: 'string', description: 'ID of the target agent' },
        taskTitle: { type: 'string', description: 'Title for the delegated task' },
        taskDescription: { type: 'string', description: 'Full context and requirements for the subtask' },
        expectation: { type: 'string', description: 'Expected result format or outcome' },
      },
      required: ['agentId', 'taskTitle', 'taskDescription', 'expectation'],
    },

    async execute(input: unknown, context?: ToolExecutionContext): Promise<ToolResult> {
      if (!isDelegateParams(input)) {
        return {
          success: false,
          output: '',
          error: 'delegate-to-agent: invalid parameters — agentId, taskTitle, taskDescription, and expectation are required',
        };
      }

      const { agentId, taskTitle, taskDescription, expectation } = input;
      const currentTaskId = context?.taskId;
      const currentAgentId = context?.agentId;

      // ── Validate target agent ────────────────────────────────────
      const targetAgent = agents.get(agentId);
      if (!targetAgent) {
        return {
          success: false,
          output: '',
          error: `delegate-to-agent: target agent "${agentId}" not found`,
        };
      }
      if (targetAgent.status === 'stopped' || targetAgent.status === 'error') {
        return {
          success: false,
          output: '',
          error: `delegate-to-agent: target agent "${agentId}" is ${targetAgent.status} — cannot accept tasks`,
        };
      }

      // ── Cycle & depth detection ──────────────────────────────────
      if (currentTaskId) {
        const chain = buildParentChain(taskGraph, currentTaskId);

        if (chain.length >= MAX_DELEGATION_DEPTH) {
          return {
            success: false,
            output: '',
            error: `delegate-to-agent: delegation chain too deep (max ${MAX_DELEGATION_DEPTH} levels)`,
          };
        }

        // Check if target agent already appears in the chain (cycle)
        for (const ancestorTask of chain) {
          if (ancestorTask.assignedAgent === agentId || ancestorTask.sourceAgentId === agentId) {
            return {
              success: false,
              output: '',
              error: `delegate-to-agent: circular delegation detected — agent "${agentId}" already appears in the delegation chain`,
            };
          }
        }
      }

      // ── Create subtask ───────────────────────────────────────────
      const subtask = taskGraph.createTask({
        title: taskTitle,
        description: `${taskDescription}\n\nExpectation: ${expectation}`,
        source: 'agent_delegation',
        parentId: currentTaskId,
        assignedAgent: agentId,
        sourceAgentId: currentAgentId,
      });

      // ── Wait for completion ──────────────────────────────────────
      return new Promise<ToolResult>((resolve) => {
        let unsub: (() => void) | null = null;
        let timedOut = false;

        const timer = setTimeout(() => {
          timedOut = true;
          if (unsub) { unsub(); }
          taskGraph.updateStatus(subtask.id, 'failed', 'Subtask timed out after 10 minutes');
          resolve({
            success: false,
            output: '',
            error: `delegate-to-agent: subtask "${subtask.id}" timed out after 10 minutes`,
          });
        }, SUBTASK_TIMEOUT_MS);

        unsub = bus.subscribe('task.status_changed', (event) => {
          if (timedOut) { return; }
          const payload = event.payload as { task: Task; newStatus: string };
          if (payload.task.id !== subtask.id) { return; }

          const newStatus = payload.newStatus;
          if (newStatus === 'completed') {
            clearTimeout(timer);
            if (unsub) { unsub(); }
            resolve({
              success: true,
              output: payload.task.result ?? `Subtask "${taskTitle}" completed successfully.`,
            });
          } else if (newStatus === 'failed') {
            clearTimeout(timer);
            if (unsub) { unsub(); }
            resolve({
              success: false,
              output: '',
              error: `delegate-to-agent: subtask failed — ${payload.task.result ?? 'unknown error'}`,
            });
          }
        });

        // Dispatch to target agent (fire and forget — result comes via bus)
        targetAgent.handleTask(subtask.id).catch((err: unknown) => {
          if (timedOut) { return; }
          clearTimeout(timer);
          if (unsub) { unsub(); }
          const msg = err instanceof Error ? err.message : String(err);
          resolve({
            success: false,
            output: '',
            error: `delegate-to-agent: agent "${agentId}" threw an error — ${msg}`,
          });
        });
      });
    },
  };
}
