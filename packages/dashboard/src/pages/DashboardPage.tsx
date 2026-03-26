import { useEffect, useRef, useState } from "react";
import { useFortSocket } from "../contexts/FortSocketContext";
import { timeAgo } from "../utils/time";
import type { Task, AgentInfo, WSMessage, Notification } from "../types";

function NotificationIcon({ type }: { type: Notification["type"] }) {
  switch (type) {
    case "task.completed": return <span>✅</span>;
    case "task.failed": return <span>❌</span>;
    case "approval.required": return <span>⚠️</span>;
    case "agent.started": return <span>🟢</span>;
    case "agent.stopped": return <span>🔴</span>;
    default: return <span>🔔</span>;
  }
}

function NotificationBell({ send, subscribe }: { send: (type: string, payload?: unknown) => void; subscribe: (type: string, handler: (msg: WSMessage) => void) => () => void }) {
  const [open, setOpen] = useState(false);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    // Load initial notifications and unread count
    send("notifications.list", { limit: 20 });
    send("notifications.unread_count");

    const unsubs = [
      subscribe("notifications.list.response", (msg: WSMessage) => {
        setNotifications((msg.payload as Notification[]) || []);
      }),
      subscribe("notifications.unread_count.response", (msg: WSMessage) => {
        setUnreadCount((msg.payload as number) ?? 0);
      }),
      subscribe("notifications.mark_all_read.response", () => {
        setNotifications((prev) => prev.map((n) => ({ ...n, read: true })));
        setUnreadCount(0);
      }),
      subscribe("notification.mark_read.response", (msg: WSMessage) => {
        const { id } = (msg.payload as { id: string }) ?? {};
        if (id) {
          setNotifications((prev) =>
            prev.map((n) => (n.id === id ? { ...n, read: true } : n))
          );
          setUnreadCount((c) => Math.max(0, c - 1));
        }
      }),
      subscribe("notification.new", (msg: WSMessage) => {
        const n = msg.payload as Notification;
        if (!n) return;
        setNotifications((prev) => [n, ...prev].slice(0, 20));
        setUnreadCount((c) => c + 1);
      }),
    ];

    return () => { unsubs.forEach((u) => u()); };
  }, [send, subscribe]);

  // Close panel on outside click
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (panelRef.current && !panelRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => { document.removeEventListener("mousedown", handler); };
  }, [open]);

  const handleBellClick = () => {
    if (!open) {
      send("notifications.list", { limit: 20 });
    }
    setOpen((v) => !v);
  };

  const handleMarkAllRead = () => {
    send("notifications.mark_all_read");
  };

  const handleMarkRead = (id: string) => {
    send("notification.mark_read", { id });
  };

  return (
    <div className="notification-bell-container" ref={panelRef}>
      <button
        className="notification-bell-btn"
        onClick={handleBellClick}
        aria-label={`Notifications${unreadCount > 0 ? ` (${unreadCount} unread)` : ""}`}
      >
        🔔
        {unreadCount > 0 && (
          <span className="notification-badge">{unreadCount > 99 ? "99+" : unreadCount}</span>
        )}
      </button>

      {open && (
        <div className="notification-panel">
          <div className="notification-panel-header">
            <span className="notification-panel-title">Notifications</span>
            {unreadCount > 0 && (
              <button className="notification-mark-all-btn" onClick={handleMarkAllRead}>
                Mark all read
              </button>
            )}
          </div>
          <div className="notification-list">
            {notifications.length === 0 ? (
              <div className="notification-empty">No notifications</div>
            ) : (
              notifications.map((n) => (
                <div
                  key={n.id}
                  className={`notification-item${n.read ? "" : " notification-item--unread"}`}
                  onClick={() => { if (!n.read) handleMarkRead(n.id); }}
                >
                  <span className="notification-icon">
                    <NotificationIcon type={n.type} />
                  </span>
                  <div className="notification-content">
                    <div className="notification-title">{n.title}</div>
                    {n.body && <div className="notification-body">{n.body}</div>}
                    <div className="notification-time">{timeAgo(n.createdAt)}</div>
                  </div>
                  {!n.read && <span className="notification-dot" />}
                </div>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  );
}

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
      subscribe("task.created", () => { send("tasks"); }),
      subscribe("task.status_changed", () => { send("tasks"); }),
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
      <div className="dashboard-header">
        <NotificationBell send={send} subscribe={subscribe} />
      </div>

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
