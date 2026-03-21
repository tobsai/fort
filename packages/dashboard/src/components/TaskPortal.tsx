import { useState } from "react";

type SubTab = "active" | "queue" | "history";

interface Task {
  id: string;
  title: string;
  status: "active" | "queued" | "done";
  agent?: string;
  timestamp: string;
}

const MOCK_TASKS: Task[] = [
  { id: "t-001", title: "Refactor authentication module", status: "active", agent: "agent-alpha", timestamp: "2m ago" },
  { id: "t-002", title: "Write integration tests for API", status: "active", agent: "agent-beta", timestamp: "5m ago" },
  { id: "t-003", title: "Update dependency versions", status: "queued", timestamp: "12m ago" },
  { id: "t-004", title: "Review PR #342 security changes", status: "queued", timestamp: "18m ago" },
  { id: "t-005", title: "Deploy staging environment", status: "queued", timestamp: "25m ago" },
  { id: "t-006", title: "Fix memory leak in worker pool", status: "done", agent: "agent-alpha", timestamp: "1h ago" },
  { id: "t-007", title: "Generate API documentation", status: "done", agent: "agent-beta", timestamp: "2h ago" },
];

function TaskPortal() {
  const [subTab, setSubTab] = useState<SubTab>("active");

  const filterMap: Record<SubTab, Task["status"]> = {
    active: "active",
    queue: "queued",
    history: "done",
  };

  const filtered = MOCK_TASKS.filter((t) => t.status === filterMap[subTab]);

  const subTabs: { key: SubTab; label: string; count: number }[] = [
    { key: "active", label: "Active", count: MOCK_TASKS.filter((t) => t.status === "active").length },
    { key: "queue", label: "Queue", count: MOCK_TASKS.filter((t) => t.status === "queued").length },
    { key: "history", label: "History", count: MOCK_TASKS.filter((t) => t.status === "done").length },
  ];

  return (
    <div className="panel">
      <div className="panel-header">
        <h2>Task Portal</h2>
      </div>

      <div className="sub-tabs">
        {subTabs.map((tab) => (
          <button
            key={tab.key}
            className={subTab === tab.key ? "active" : ""}
            onClick={() => setSubTab(tab.key)}
          >
            {tab.label} ({tab.count})
          </button>
        ))}
      </div>

      <ul className="task-list">
        {filtered.map((task) => (
          <li key={task.id} className="task-item">
            <span className={`status-dot ${task.status === "active" ? "active" : task.status === "queued" ? "queued" : "done"}`} />
            <div className="task-info">
              <div className="task-title">{task.title}</div>
              <div className="task-meta">
                {task.id}
                {task.agent && <> &middot; {task.agent}</>}
                {" "}&middot; {task.timestamp}
              </div>
            </div>
          </li>
        ))}
        {filtered.length === 0 && (
          <li className="empty-state">No tasks</li>
        )}
      </ul>
    </div>
  );
}

export default TaskPortal;
