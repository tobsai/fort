export interface AgentConfig {
  id: string;
  name: string;
  type: string;
  description: string;
  capabilities: string[];
  memoryPartition: string;
  isDefault?: boolean;
}

export interface AgentInfo {
  config: AgentConfig;
  status: "running" | "paused" | "stopped" | "error";
  currentTaskId: string | null;
  startedAt: string;
  taskCount: number;
  errorCount: number;
  soul?: string;
  emoji?: string;
  isDefault?: boolean;
}

export type TaskStatus =
  | "created"
  | "in_progress"
  | "completed"
  | "failed"
  | "blocked"
  | "needs_review";

export interface Task {
  id: string;
  shortId: string;
  title: string;
  description: string;
  status: TaskStatus;
  source: string;
  assignedAgent?: string;
  result?: string;
  metadata: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
}

export interface ToolCallEvent {
  id: string;
  toolName: string;
  input?: unknown;
  result?: {
    success: boolean;
    output: string;
    error?: string;
  };
  denied: boolean;
  denialReason?: string;
  durationMs?: number;
  taskId?: string;
  agentId?: string;
  calledAt?: string;
}

export interface ChatMessage {
  role: "user" | "agent" | "tool";
  text: string;
  ts: number;
  task?: {
    shortId: string;
    title: string;
    status: string;
  } | null;
  toolCall?: ToolCallEvent;
  toolEventType?: "tool.executed" | "tool.denied" | "tool.error";
}

export interface FortState {
  agents: AgentInfo[];
  activeTasks: number;
  totalTasks: number;
  memoryStats: { nodeCount: number };
}

export interface WSMessage {
  id: string;
  type: string;
  payload?: unknown;
  error?: string;
}

export interface Thread {
  id: string;
  name: string;
  description: string;
  taskId: string;
  assignedAgent: string | null;
  status: 'active' | 'paused' | 'resolved';
  lastActiveAt: string;
  createdAt: string;
}

export interface ThreadMessage {
  id: string;
  threadId: string;
  role: 'user' | 'agent' | 'system';
  content: string;
  agentId: string | null;
  createdAt: string;
}
