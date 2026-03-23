import { useEffect, useState } from "react";
import { useFortSocket } from "../contexts/FortSocketContext";
import { timeAgo } from "../utils/time";
import type { Task, AgentInfo, WSMessage } from "../types";

export default function DashboardPage() {
  const { state, send, subscribe } = useFortSocket();
  const [tasks, setTasks] = useState<Task[]>([]);
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [doneExpanded, setDoneExpanded] = useState(false);

  useEffect(() => {
    const unsubs = [
      subscribe("tasks.response", (msg: WSMessage) => {
        setTasks((msg.payload as Task[]) || []);
      }),
      subscribe("agents.response", (msg: WSMessage) => {
        setAgents((msg.payload as AgentInfo[]) || []);
      }),
      subscribe("task.created", () => send("tasks")),
      subscribe("task.status_changed", () => send("tasks")),
    ];
    send("tasks");
    send("agents");
    return () => unsubs.forEach((u) => u());
  }, [send, subscribe]);

  const todo = tasks.filter((t) => t.status === "created");
  const inProgress = tasks.filter((t) => t.status === "in_progress");
  const done = tasks.filter(
    (t) => t.status === "completed" || t.status === "failed",
  );

  const getAgentEmoji = (name?: string) => {
    if (!name) return "";
    const agent = agents.find(
      (a) => a.config.id === name || a.config.name === name,
    );
    return agent?.emoji || "🤖";
  };

  const renderCard = (t: Task) => (
    <div key={t.id} className="kanban-card">
      <div className="kanban-card-title">{t.title}</div>
      <div className="kanban-card-meta">
        <span className="kanban-card-id">{t.shortId}</span>
        {t.assignedAgent && (
          <span className="kanban-card-agent">
            {getAgentEmoji(t.assignedAgent)} {t.assignedAgent}
          </span>
        )}
        <span className="kanban-card-time">{timeAgo(t.createdAt)}</span>
      </div>
    </div>
  );

  return (
    <div className="dashboard-page">
      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-value">{state?.agents.length ?? 0}</div>
          <div className="stat-label">Agents</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{state?.activeTasks ?? 0}</div>
          <div className="stat-label">Active Tasks</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{state?.totalTasks ?? 0}</div>
          <div className="stat-label">Total Tasks</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{state?.memoryStats.nodeCount ?? 0}</div>
          <div className="stat-label">Memory Nodes</div>
        </div>
      </div>

      <div className="kanban">
        <div className="kanban-column">
          <div className="kanban-column-header">
            To Do <span className="kanban-count">{todo.length}</span>
          </div>
          {todo.map(renderCard)}
        </div>
        <div className="kanban-column">
          <div className="kanban-column-header">
            In Progress <span className="kanban-count">{inProgress.length}</span>
          </div>
          {inProgress.map(renderCard)}
        </div>
        <div className="kanban-column">
          <div className="kanban-column-header">
            Done <span className="kanban-count">{done.length}</span>
            {done.length > 10 && (
              <button
                className="kanban-toggle"
                onClick={() => setDoneExpanded(!doneExpanded)}
              >
                {doneExpanded ? "Show less" : "Show all"}
              </button>
            )}
          </div>
          {(doneExpanded ? done : done.slice(0, 10)).map(renderCard)}
        </div>
      </div>
    </div>
  );
}
