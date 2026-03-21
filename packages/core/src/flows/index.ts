/**
 * Flow Engine — Deterministic Workflow Execution
 *
 * Flows are YAML-defined structured workflows. The LLM is only invoked
 * for steps explicitly marked as `llm` — everything else is deterministic.
 * Each flow execution creates a task in TaskGraph for full transparency.
 */

import { readFileSync } from 'node:fs';
import { v4 as uuid } from 'uuid';
import * as yaml from 'yaml';
import type { ModuleBus } from '../module-bus/index.js';
import type { TaskGraph } from '../task-graph/index.js';
import type { LLMClient } from '../llm/index.js';
import type {
  DiagnosticResult,
  FlowDefinition,
  FlowExecution,
  FlowExecutionStatus,
  FlowStep,
  FlowStepResult,
} from '../types.js';

export type ActionHandler = (
  params: Record<string, unknown>,
  context: Record<string, unknown>,
) => Promise<unknown>;

export class FlowEngine {
  private flows: Map<string, FlowDefinition> = new Map();
  private executions: Map<string, FlowExecution> = new Map();
  private actionHandlers: Map<string, ActionHandler> = new Map();
  private bus: ModuleBus;
  private taskGraph: TaskGraph;
  private llm: LLMClient | null = null;

  constructor(bus: ModuleBus, taskGraph: TaskGraph) {
    this.bus = bus;
    this.taskGraph = taskGraph;
  }

  /**
   * Attach an LLM client for flow steps that require reasoning.
   */
  setLLMClient(llm: LLMClient): void {
    this.llm = llm;
  }

  // ─── Action Handler Registration ────────────────────────────────

  registerAction(toolName: string, handler: ActionHandler): void {
    this.actionHandlers.set(toolName, handler);
  }

  // ─── Flow Registration ──────────────────────────────────────────

  loadFlow(yamlPath: string): FlowDefinition {
    const content = readFileSync(yamlPath, 'utf-8');
    const definition = yaml.parse(content) as FlowDefinition;
    return this.registerFlow(definition);
  }

  registerFlow(definition: FlowDefinition): FlowDefinition {
    if (!definition.id || !definition.name || !definition.steps) {
      throw new Error('Flow definition must include id, name, and steps');
    }
    if (!definition.onError) {
      definition.onError = 'abort';
    }
    this.flows.set(definition.id, definition);
    this.bus.publish('flow.registered', 'flow-engine', {
      flowId: definition.id,
      name: definition.name,
    });
    return definition;
  }

  // ─── Flow Execution ─────────────────────────────────────────────

  async executeFlow(
    flowId: string,
    context: Record<string, unknown> = {},
  ): Promise<FlowExecution> {
    const flow = this.flows.get(flowId);
    if (!flow) throw new Error(`Flow not found: ${flowId}`);

    // Create a task for transparency
    const task = this.taskGraph.createTask({
      title: `Flow: ${flow.name}`,
      description: flow.description,
      source: 'background',
      metadata: { flowId },
    });

    const execution: FlowExecution = {
      id: uuid(),
      flowId,
      status: 'running',
      startedAt: new Date(),
      completedAt: null,
      stepResults: [],
      context: { ...context },
      taskId: task.id,
    };

    this.executions.set(execution.id, execution);
    this.taskGraph.updateStatus(task.id, 'in_progress');

    await this.bus.publish('flow.started', 'flow-engine', {
      executionId: execution.id,
      flowId,
    });

    try {
      await this.executeSteps(flow.steps, execution, flow.onError);

      if (execution.status === 'running') {
        execution.status = 'completed';
      }
    } catch (err) {
      execution.status = 'failed';
      execution.error = err instanceof Error ? err.message : String(err);
    }

    execution.completedAt = new Date();

    const taskStatus = execution.status === 'completed' ? 'completed' : 'failed';
    this.taskGraph.updateStatus(task.id, taskStatus, execution.error);

    await this.bus.publish('flow.completed', 'flow-engine', {
      executionId: execution.id,
      flowId,
      status: execution.status,
    });

    return execution;
  }

  private async executeSteps(
    steps: FlowStep[],
    execution: FlowExecution,
    onError: FlowDefinition['onError'],
  ): Promise<void> {
    for (const step of steps) {
      if (execution.status !== 'running') break;

      const startedAt = new Date();
      let result: FlowStepResult;

      try {
        const output = await this.executeStep(step, execution);
        result = {
          stepId: step.id,
          stepName: step.name,
          status: 'completed',
          output,
          startedAt,
          completedAt: new Date(),
        };

        // Store step output in context for subsequent steps
        execution.context[step.id] = output;
      } catch (err) {
        const errorMsg = err instanceof Error ? err.message : String(err);

        result = {
          stepId: step.id,
          stepName: step.name,
          status: 'failed',
          output: null,
          error: errorMsg,
          startedAt,
          completedAt: new Date(),
        };

        switch (onError) {
          case 'abort':
            execution.stepResults.push(result);
            execution.status = 'aborted';
            execution.error = `Step "${step.name}" failed: ${errorMsg}`;
            await this.bus.publish('flow.step_failed', 'flow-engine', {
              executionId: execution.id,
              stepId: step.id,
              error: errorMsg,
              policy: 'abort',
            });
            return;

          case 'skip':
            result.status = 'skipped';
            await this.bus.publish('flow.step_failed', 'flow-engine', {
              executionId: execution.id,
              stepId: step.id,
              error: errorMsg,
              policy: 'skip',
            });
            break;

          case 'retry': {
            // Single retry attempt
            try {
              const retryOutput = await this.executeStep(step, execution);
              result = {
                stepId: step.id,
                stepName: step.name,
                status: 'completed',
                output: retryOutput,
                startedAt,
                completedAt: new Date(),
              };
              execution.context[step.id] = retryOutput;
            } catch (retryErr) {
              const retryErrorMsg = retryErr instanceof Error ? retryErr.message : String(retryErr);
              result.error = retryErrorMsg;
              execution.status = 'failed';
              execution.error = `Step "${step.name}" failed after retry: ${retryErrorMsg}`;
              execution.stepResults.push(result);
              return;
            }
            break;
          }
        }
      }

      execution.stepResults.push(result);
    }
  }

  private async executeStep(
    step: FlowStep,
    execution: FlowExecution,
  ): Promise<unknown> {
    switch (step.type) {
      case 'action':
        return this.executeAction(step.tool, step.params ?? {}, execution.context);

      case 'condition':
        return this.executeCondition(step, execution);

      case 'transform':
        return this.executeTransform(step.expression, execution.context);

      case 'llm':
        if (this.llm && this.llm.isConfigured) {
          // Resolve prompt — interpolate context variables
          const resolvedPrompt = this.interpolate(step.prompt, execution.context);
          const response = await this.llm.complete({
            messages: [{ role: 'user', content: resolvedPrompt }],
            model: step.model as any,
            taskId: execution.taskId,
            injectBehaviors: true,
          });
          await this.bus.publish('flow.llm_completed', 'flow-engine', {
            executionId: execution.id,
            stepId: step.id,
            model: response.model,
            tokens: response.totalTokens,
          });
          return { content: response.content, model: response.model, tokens: response.totalTokens };
        }
        // Fallback: emit event for external handling
        await this.bus.publish('flow.llm_requested', 'flow-engine', {
          executionId: execution.id,
          stepId: step.id,
          prompt: step.prompt,
          model: step.model,
        });
        return { placeholder: true, prompt: step.prompt };

      case 'parallel':
        return this.executeParallel(step.branches, execution);

      case 'notify':
        await this.bus.publish(step.eventType, 'flow-engine', {
          ...step.payload,
          _flowExecutionId: execution.id,
        });
        return { notified: step.eventType };

      default:
        throw new Error(`Unknown step type: ${(step as FlowStep).type}`);
    }
  }

  private async executeAction(
    toolName: string,
    params: Record<string, unknown>,
    context: Record<string, unknown>,
  ): Promise<unknown> {
    const handler = this.actionHandlers.get(toolName);
    if (!handler) {
      throw new Error(`No action handler registered for tool: ${toolName}`);
    }
    return handler(params, context);
  }

  private async executeCondition(
    step: FlowStep & { type: 'condition' },
    execution: FlowExecution,
  ): Promise<unknown> {
    const result = this.evaluateExpression(step.expression, execution.context);

    if (result) {
      await this.executeSteps(step.thenSteps, execution, 'abort');
      return { branch: 'then', conditionResult: result };
    } else if (step.elseSteps && step.elseSteps.length > 0) {
      await this.executeSteps(step.elseSteps, execution, 'abort');
      return { branch: 'else', conditionResult: result };
    }

    return { branch: 'none', conditionResult: result };
  }

  private executeTransform(
    expression: string,
    context: Record<string, unknown>,
  ): unknown {
    return this.evaluateExpression(expression, context);
  }

  private async executeParallel(
    branches: FlowStep[][],
    execution: FlowExecution,
  ): Promise<unknown[]> {
    const results = await Promise.all(
      branches.map(async (branch) => {
        const branchResults: unknown[] = [];
        for (const step of branch) {
          const output = await this.executeStep(step, execution);
          branchResults.push(output);
          execution.context[step.id] = output;
        }
        return branchResults;
      }),
    );
    return results;
  }

  private interpolate(template: string, context: Record<string, unknown>): string {
    return template.replace(/\{\{(\w+(?:\.\w+)*)\}\}/g, (_match, path: string) => {
      const parts = path.split('.');
      let value: unknown = context;
      for (const part of parts) {
        if (value != null && typeof value === 'object') {
          value = (value as Record<string, unknown>)[part];
        } else {
          return _match; // leave unresolved
        }
      }
      return value != null ? String(value) : _match;
    });
  }

  private evaluateExpression(
    expression: string,
    context: Record<string, unknown>,
  ): unknown {
    // Create a function that evaluates the expression with context as the scope
    const fn = new Function('context', `with(context) { return (${expression}); }`);
    return fn(context);
  }

  // ─── Queries ────────────────────────────────────────────────────

  getFlowStatus(executionId: string): FlowExecution {
    const execution = this.executions.get(executionId);
    if (!execution) throw new Error(`Flow execution not found: ${executionId}`);
    return execution;
  }

  listFlows(): FlowDefinition[] {
    return Array.from(this.flows.values());
  }

  listExecutions(flowId?: string): FlowExecution[] {
    const all = Array.from(this.executions.values());
    if (flowId) {
      return all.filter((e) => e.flowId === flowId);
    }
    return all;
  }

  getFlow(flowId: string): FlowDefinition | undefined {
    return this.flows.get(flowId);
  }

  // ─── Diagnostics ───────────────────────────────────────────────

  diagnose(): DiagnosticResult {
    const checks = [];

    const flowCount = this.flows.size;
    const executionCount = this.executions.size;
    const failedExecutions = Array.from(this.executions.values()).filter(
      (e) => e.status === 'failed' || e.status === 'aborted',
    );

    checks.push({
      name: 'Flow registry',
      passed: true,
      message: `${flowCount} flows registered`,
    });

    checks.push({
      name: 'Execution history',
      passed: true,
      message: `${executionCount} total executions`,
    });

    if (failedExecutions.length > 0) {
      checks.push({
        name: 'Failed executions',
        passed: false,
        message: `${failedExecutions.length} executions failed or aborted`,
        details: {
          failedIds: failedExecutions.map((e) => e.id),
        },
      });
    } else {
      checks.push({
        name: 'Execution health',
        passed: true,
        message: 'No failed executions',
      });
    }

    checks.push({
      name: 'Action handlers',
      passed: this.actionHandlers.size > 0 || this.flows.size === 0,
      message: `${this.actionHandlers.size} action handlers registered`,
    });

    return {
      module: 'flow-engine',
      status: checks.every((c) => c.passed) ? 'healthy' : 'degraded',
      checks,
    };
  }
}
