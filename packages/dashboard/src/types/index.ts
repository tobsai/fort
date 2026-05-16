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
  sourceAgentId?: string;
  parentId?: string;
  result?: string;
  metadata: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
  completedAt?: string;
  subtasks?: Task[];
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

export interface PlanSubtask {
  id: string;
  shortId: string;
  title: string;
  status: "created" | "pending" | "in_progress" | "blocked" | "needs_review" | "completed" | "failed";
}

export interface ChatMessage {
  role: "user" | "agent" | "tool" | "plan" | "classification";
  text: string;
  ts: number;
  task?: {
    shortId: string;
    title: string;
    status: string;
  } | null;
  toolCall?: ToolCallEvent;
  toolEventType?: "tool.executed" | "tool.denied" | "tool.error";
  /** Present on role:"plan" messages — list of subtasks and their live status. */
  plan?: {
    parentTaskId: string;
    summary?: string;
    subtasks: PlanSubtask[];
  };
  /** Present on role:"classification" messages — Triager verdict + yes/no training affordance. */
  classification?: {
    taskId: string;
    classifiedAs: "task" | "question";
    confidence: number;
    summary: string;
    /** "pending" → buttons visible; "confirmed" → user said yes; "corrected" → user flipped it */
    feedback: "pending" | "confirmed" | "corrected";
  };
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

export type NotificationType =
  | 'task.completed'
  | 'task.failed'
  | 'approval.required'
  | 'agent.started'
  | 'agent.stopped';

export interface Notification {
  id: string;
  type: NotificationType;
  title: string;
  body: string | null;
  entityType: string | null;
  entityId: string | null;
  read: boolean;
  createdAt: string;
}
