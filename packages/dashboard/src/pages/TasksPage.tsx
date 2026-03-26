import { useEffect, useState, useCallback } from "react";
import { useFortSocket } from "../contexts/FortSocketContext";
import type { Task, TaskStatus } from "../types";
import { timeAgo } from "../utils/time";

const STATUS_OPTIONS: Array<{ value: string; label: string }> = [
  { value: "", label: "All" },
  { value: "created", label: "Created" },
  { value: "in_progress", label: "In Progress" },
  { value: "completed", label: "Completed" },
  { value: "failed", label: "Failed" },
  { value: "blocked", label: "Blocked" },
  { value: "needs_review", label: "Needs Review" },
];

const STATUS_COLORS: Record<TaskStatus, string> = {
  created: "#6c757d",
  in_progress: "#0d6efd",
  completed: "#198754",
  failed: "#dc3545",
  blocked: "#fd7e14",
  needs_review: "#ffc107",
};

function durationMs(task: Task): number | null {
  if (!task.completedAt) return null;
  return new Date(task.completedAt).getTime() - new Date(task.createdAt).getTime();
}

function fmtDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)}s`;
  return `${Math.round(ms / 60_000)}m`;
}

export default function TasksPage() {
  const { send, subscribe } = useFortSocket();
  const [tasks, setTasks] = useState<Task[]>([]);
  const [statusFilter, setStatusFilter] = useState("");
  const [agentFilter, setAgentFilter] = useState("");
  const [offset, setOffset] = useState(0);
  const [expanded, setExpanded] = useState<string | null>(null);
  const PAGE_SIZE = 50;

  const fetchTasks = useCallback(() => {
    send("tasks.query", {
      status: statusFilter || undefined,
      assignedAgent: agentFilter || undefined,
      limit: PAGE_SIZE,
      offset,
    });
  }, [send, statusFilter, agentFilter, offset]);

  useEffect(() => {
    fetchTasks();
  }, [fetchTasks]);

  useEffect(() => {
    return subscribe("tasks.query.response", (msg) => {
      setTasks((msg.payload as Task[]) ?? []);
    });
  }, [subscribe]);

  // Refresh on live task changes
  useEffect(() => {
    const unsub1 = subscribe("task.created", () => { fetchTasks(); });
    const unsub2 = subscribe("task.status_changed", () => { fetchTasks(); });
    return () => { unsub1(); unsub2(); };
  }, [subscribe, fetchTasks]);

  function handleFilterChange() {
    setOffset(0);
    fetchTasks();
  }

  return (
    <div className="tasks-page" style={{ padding: "1.5rem", maxWidth: 1100, margin: "0 auto" }}>
      <h2 style={{ marginBottom: "1rem" }}>Task History</h2>

      {/* Filters */}
      <div style={{ display: "flex", gap: "1rem", marginBottom: "1.25rem", flexWrap: "wrap" }}>
        <select
          value={statusFilter}
          onChange={(e) => { setStatusFilter(e.target.value); setOffset(0); }}
          style={{ padding: "0.4rem 0.75rem", borderRadius: 6, border: "1px solid #333", background: "#1e1e28", color: "#eee" }}
        >
          {STATUS_OPTIONS.map((o) => (
            <option key={o.value} value={o.value}>{o.label}</option>
          ))}
        </select>
        <input
          type="text"
          placeholder="Filter by agent ID..."
          value={agentFilter}
          onChange={(e) => { setAgentFilter(e.target.value); setOffset(0); }}
          onKeyDown={(e) => { if (e.key === "Enter") handleFilterChange(); }}
          style={{ padding: "0.4rem 0.75rem", borderRadius: 6, border: "1px solid #333", background: "#1e1e28", color: "#eee", minWidth: 200 }}
        />
        <button
          onClick={() => { setStatusFilter(""); setAgentFilter(""); setOffset(0); }}
          style={{ padding: "0.4rem 0.75rem", borderRadius: 6, border: "1px solid #555", background: "#2a2a38", color: "#aaa", cursor: "pointer" }}
        >
          Clear
        </button>
      </div>

      {/* Table */}
      <div style={{ overflowX: "auto" }}>
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
          <thead>
            <tr style={{ textAlign: "left", borderBottom: "1px solid #333", color: "#888" }}>
              <th style={{ padding: "0.5rem 0.75rem" }}>ID</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Title</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Agent</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Status</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Duration</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Created</th>
            </tr>
          </thead>
          <tbody>
            {tasks.length === 0 && (
              <tr>
                <td colSpan={6} style={{ padding: "2rem", textAlign: "center", color: "#666" }}>
                  No tasks found.
                </td>
              </tr>
            )}
            {tasks.map((task) => {
              const dur = durationMs(task);
              const isExpanded = expanded === task.id;
              return [
                <tr
                  key={task.id}
                  onClick={() => setExpanded(isExpanded ? null : task.id)}
                  style={{
                    cursor: "pointer",
                    borderBottom: "1px solid #222",
                    background: isExpanded ? "#1a1a26" : "transparent",
                  }}
                >
                  <td style={{ padding: "0.5rem 0.75rem", color: "#888", fontFamily: "monospace" }}>
                    {task.shortId || task.id.slice(0, 8)}
                  </td>
                  <td style={{ padding: "0.5rem 0.75rem", maxWidth: 320, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {task.title}
                  </td>
                  <td style={{ padding: "0.5rem 0.75rem", color: "#aaa", fontFamily: "monospace", fontSize: 11 }}>
                    {task.assignedAgent ? task.assignedAgent.slice(0, 12) : "—"}
                  </td>
                  <td style={{ padding: "0.5rem 0.75rem" }}>
                    <span style={{
                      padding: "2px 8px",
                      borderRadius: 12,
                      background: STATUS_COLORS[task.status] + "33",
                      color: STATUS_COLORS[task.status],
                      fontSize: 11,
                      fontWeight: 600,
                    }}>
                      {task.status}
                    </span>
                  </td>
                  <td style={{ padding: "0.5rem 0.75rem", color: "#888" }}>
                    {dur !== null ? fmtDuration(dur) : "—"}
                  </td>
                  <td style={{ padding: "0.5rem 0.75rem", color: "#888" }}>
                    {timeAgo(task.createdAt)}
                  </td>
                </tr>,
                isExpanded && (
                  <tr key={`${task.id}-detail`} style={{ background: "#1a1a26" }}>
                    <td colSpan={6} style={{ padding: "0.75rem 1.5rem 1rem", borderBottom: "1px solid #333" }}>
                      {task.description && (
                        <div style={{ marginBottom: "0.5rem", color: "#bbb" }}>
                          <strong style={{ color: "#888" }}>Description: </strong>{task.description}
                        </div>
                      )}
                      {task.result && (
                        <div style={{ marginBottom: "0.5rem" }}>
                          <strong style={{ color: "#888" }}>Result: </strong>
                          <span style={{ color: "#ccc" }}>{task.result}</span>
                        </div>
                      )}
                      {task.metadata?.statusReason && (
                        <div style={{ color: "#ffc107", fontSize: 12 }}>
                          <strong>Reason: </strong>{String(task.metadata.statusReason)}
                        </div>
                      )}
                    </td>
                  </tr>
                ),
              ];
            })}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <div style={{ display: "flex", gap: "0.75rem", marginTop: "1rem", alignItems: "center" }}>
        <button
          disabled={offset === 0}
          onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
          style={{ padding: "0.4rem 0.9rem", borderRadius: 6, border: "1px solid #444", background: "#2a2a38", color: "#ccc", cursor: offset === 0 ? "not-allowed" : "pointer", opacity: offset === 0 ? 0.5 : 1 }}
        >
          ← Prev
        </button>
        <span style={{ color: "#888", fontSize: 13 }}>
          {offset + 1}–{offset + tasks.length}
        </span>
        <button
          disabled={tasks.length < PAGE_SIZE}
          onClick={() => setOffset(offset + PAGE_SIZE)}
          style={{ padding: "0.4rem 0.9rem", borderRadius: 6, border: "1px solid #444", background: "#2a2a38", color: "#ccc", cursor: tasks.length < PAGE_SIZE ? "not-allowed" : "pointer", opacity: tasks.length < PAGE_SIZE ? 0.5 : 1 }}
        >
          Next →
        </button>
      </div>
    </div>
  );
}
