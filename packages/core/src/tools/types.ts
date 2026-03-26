/**
 * Tool Execution Types — FortTool Interface + ToolResult
 *
 * Defines the executable tool interface that powers Fort's tool runtime.
 * FortTool wraps a deterministic capability with a permission tier and
 * a structured execute() contract. The registry holds metadata (ToolDefinition);
 * these types describe live, callable tools.
 */

import type { ActionTier } from '../types.js';

export interface FortTool {
  /** Unique identifier for this tool (kebab-case) */
  name: string;
  /** Human-readable description shown to the LLM */
  description: string;
  /** JSON Schema for the input parameters */
  inputSchema: object;
  /** Permission tier: 1=auto, 2=draft, 3=approve, 4=never */
  tier: ActionTier;
  /** Execute the tool with validated input */
  execute(input: unknown): Promise<ToolResult>;
}

export interface ToolResult {
  /** Whether execution succeeded */
  success: boolean;
  /** Text result shown to the LLM */
  output: string;
  /** Optional structured data produced by the tool */
  artifacts?: unknown[];
  /** Error message when success is false */
  error?: string;
}

export interface ToolCallLog {
  /** Unique log entry ID */
  id: string;
  /** Tool name */
  toolName: string;
  /** Raw input passed to the tool */
  input: unknown;
  /** Result of the execution */
  result: ToolResult;
  /** Execution duration in milliseconds */
  durationMs: number;
  /** Whether execution was denied by permissions */
  denied: boolean;
  /** Denial reason when denied is true */
  denialReason?: string;
  /** Task context for cost tracking */
  taskId?: string;
  /** Agent that invoked the tool */
  agentId?: string;
  /** Timestamp of the call */
  calledAt: Date;
}
