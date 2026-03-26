import { useEffect, useState, useCallback } from "react";
import { useFortSocket } from "../contexts/FortSocketContext";
import { timeAgo } from "../utils/time";

interface ScheduledTask {
  id: string;
  name: string;
  description: string;
  agentId: string;
  scheduleType: "cron" | "interval";
  scheduleValue: string;
  taskTitle: string;
  taskDescription: string;
  enabled: boolean;
  lastRunAt: string | null;
  nextRunAt: string | null;
  runCount: number;
  createdAt: string;
  updatedAt: string;
}

const INTERVAL_OPTIONS = [
  { value: "15m", label: "Every 15 minutes" },
  { value: "30m", label: "Every 30 minutes" },
  { value: "1h", label: "Every hour" },
  { value: "6h", label: "Every 6 hours" },
  { value: "12h", label: "Every 12 hours" },
  { value: "1d", label: "Every day" },
];

interface AgentInfo {
  config: { id: string; name: string };
}

const inputStyle: React.CSSProperties = {
  padding: "0.4rem 0.75rem",
  borderRadius: 6,
  border: "1px solid #333",
  background: "#1e1e28",
  color: "#eee",
  width: "100%",
  boxSizing: "border-box",
};

const labelStyle: React.CSSProperties = {
  display: "block",
  marginBottom: "0.25rem",
  color: "#aaa",
  fontSize: 12,
};

function FieldRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: "0.85rem" }}>
      <label style={labelStyle}>{label}</label>
      {children}
    </div>
  );
}

export default function SchedulesPage() {
  const { send, subscribe } = useFortSocket();
  const [schedules, setSchedules] = useState<ScheduledTask[]>([]);
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [showModal, setShowModal] = useState(false);
  const [formError, setFormError] = useState("");

  // Form state
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [agentId, setAgentId] = useState("");
  const [scheduleType, setScheduleType] = useState<"cron" | "interval">("interval");
  const [scheduleValue, setScheduleValue] = useState("30m");
  const [taskTitle, setTaskTitle] = useState("");
  const [taskDescription, setTaskDescription] = useState("");

  const fetchSchedules = useCallback(() => {
    send("schedules.list");
  }, [send]);

  useEffect(() => {
    fetchSchedules();
    send("agents.list");
  }, [fetchSchedules, send]);

  useEffect(() => {
    return subscribe("schedules.list.response", (msg) => {
      setSchedules((msg.payload as ScheduledTask[]) ?? []);
    });
  }, [subscribe]);

  useEffect(() => {
    return subscribe("agents.response", (msg) => {
      setAgents((msg.payload as AgentInfo[]) ?? []);
    });
  }, [subscribe]);

  function resetForm() {
    setName("");
    setDescription("");
    setAgentId(agents[0]?.config.id ?? "");
    setScheduleType("interval");
    setScheduleValue("30m");
    setTaskTitle("");
    setTaskDescription("");
    setFormError("");
  }

  function openModal() {
    resetForm();
    setShowModal(true);
  }

  function handleCreate() {
    if (!name.trim()) { setFormError("Name is required"); return; }
    if (!agentId) { setFormError("Select an agent"); return; }
    if (!scheduleValue.trim()) { setFormError("Schedule value is required"); return; }
    if (!taskTitle.trim()) { setFormError("Task title is required"); return; }
    setFormError("");

    const payload = {
      name: name.trim(),
      description,
      agentId,
      scheduleType,
      scheduleValue: scheduleValue.trim(),
      taskTitle: taskTitle.trim(),
      taskDescription,
    };

    // Listen for create response once
    const unsub = subscribe("schedule.create.response", () => {
      unsub();
      setShowModal(false);
      fetchSchedules();
    });
    const unsubErr = subscribe("error", (msg) => {
      if (String((msg.payload as any)?.error ?? msg.error ?? "").includes("schedule")) {
        unsubErr();
        setFormError(String((msg.payload as any)?.error ?? msg.error ?? "Unknown error"));
      }
    });

    send("schedule.create", payload);
  }

  function handlePause(id: string) {
    send("schedule.pause", { id });
    setTimeout(fetchSchedules, 200);
  }

  function handleResume(id: string) {
    send("schedule.resume", { id });
    setTimeout(fetchSchedules, 200);
  }

  function handleRunNow(id: string) {
    send("schedule.run_now", { id });
    setTimeout(fetchSchedules, 200);
  }

  function handleDelete(id: string) {
    if (!confirm("Delete this schedule?")) return;
    send("schedule.delete", { id });
    setTimeout(fetchSchedules, 200);
  }

  function fmtSchedule(s: ScheduledTask) {
    if (s.scheduleType === "interval") {
      const opt = INTERVAL_OPTIONS.find((o) => o.value === s.scheduleValue);
      return opt ? opt.label : `Every ${s.scheduleValue}`;
    }
    return s.scheduleValue;
  }

  return (
    <div style={{ padding: "1.5rem", maxWidth: 1100, margin: "0 auto" }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "1.25rem" }}>
        <h2 style={{ margin: 0 }}>Schedules</h2>
        <button
          onClick={openModal}
          style={{ padding: "0.45rem 1rem", borderRadius: 6, border: "none", background: "#6c5ce7", color: "#fff", cursor: "pointer", fontWeight: 600 }}
        >
          + New Schedule
        </button>
      </div>

      {/* Table */}
      <div style={{ overflowX: "auto" }}>
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
          <thead>
            <tr style={{ textAlign: "left", borderBottom: "1px solid #333", color: "#888" }}>
              <th style={{ padding: "0.5rem 0.75rem" }}>Name</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Agent</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Schedule</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Last Run</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Next Run</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Runs</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Status</th>
              <th style={{ padding: "0.5rem 0.75rem" }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {schedules.length === 0 && (
              <tr>
                <td colSpan={8} style={{ padding: "2rem", textAlign: "center", color: "#666" }}>
                  No schedules. Create one to get started.
                </td>
              </tr>
            )}
            {schedules.map((s) => (
              <tr key={s.id} style={{ borderBottom: "1px solid #222" }}>
                <td style={{ padding: "0.5rem 0.75rem" }}>
                  <div style={{ fontWeight: 500 }}>{s.name}</div>
                  {s.description && <div style={{ color: "#666", fontSize: 11 }}>{s.description}</div>}
                </td>
                <td style={{ padding: "0.5rem 0.75rem", color: "#aaa", fontFamily: "monospace", fontSize: 11 }}>
                  {s.agentId.slice(0, 14)}
                </td>
                <td style={{ padding: "0.5rem 0.75rem", color: "#ccc" }}>
                  {fmtSchedule(s)}
                </td>
                <td style={{ padding: "0.5rem 0.75rem", color: "#888" }}>
                  {s.lastRunAt ? timeAgo(s.lastRunAt) : "—"}
                </td>
                <td style={{ padding: "0.5rem 0.75rem", color: "#888" }}>
                  {s.nextRunAt && s.enabled ? timeAgo(s.nextRunAt) : "—"}
                </td>
                <td style={{ padding: "0.5rem 0.75rem", color: "#888" }}>
                  {s.runCount}
                </td>
                <td style={{ padding: "0.5rem 0.75rem" }}>
                  <span style={{
                    padding: "2px 8px",
                    borderRadius: 12,
                    background: s.enabled ? "#19875433" : "#6c757d33",
                    color: s.enabled ? "#198754" : "#6c757d",
                    fontSize: 11,
                    fontWeight: 600,
                  }}>
                    {s.enabled ? "active" : "paused"}
                  </span>
                </td>
                <td style={{ padding: "0.5rem 0.75rem" }}>
                  <div style={{ display: "flex", gap: "0.4rem" }}>
                    {s.enabled ? (
                      <button onClick={() => handlePause(s.id)} style={actionBtnStyle("#fd7e14")}>Pause</button>
                    ) : (
                      <button onClick={() => handleResume(s.id)} style={actionBtnStyle("#198754")}>Resume</button>
                    )}
                    <button onClick={() => handleRunNow(s.id)} style={actionBtnStyle("#0d6efd")}>Run Now</button>
                    <button onClick={() => handleDelete(s.id)} style={actionBtnStyle("#dc3545")}>Delete</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Create Modal */}
      {showModal && (
        <div style={{
          position: "fixed", inset: 0, background: "rgba(0,0,0,0.7)",
          display: "flex", alignItems: "center", justifyContent: "center", zIndex: 1000,
        }}>
          <div style={{
            background: "#16161f", border: "1px solid #333", borderRadius: 10,
            padding: "1.75rem", width: 480, maxWidth: "95vw",
          }}>
            <h3 style={{ margin: "0 0 1.25rem", color: "#eee" }}>New Schedule</h3>

            <FieldRow label="Name *">
              <input value={name} onChange={(e) => setName(e.target.value)} style={inputStyle} placeholder="Daily email check" />
            </FieldRow>

            <FieldRow label="Description">
              <input value={description} onChange={(e) => setDescription(e.target.value)} style={inputStyle} placeholder="Optional description" />
            </FieldRow>

            <FieldRow label="Agent *">
              <select value={agentId} onChange={(e) => setAgentId(e.target.value)} style={inputStyle}>
                <option value="">Select agent…</option>
                {agents.map((a) => (
                  <option key={a.config.id} value={a.config.id}>{a.config.name}</option>
                ))}
              </select>
            </FieldRow>

            <FieldRow label="Schedule type">
              <div style={{ display: "flex", gap: "0.5rem" }}>
                {(["interval", "cron"] as const).map((t) => (
                  <button
                    key={t}
                    onClick={() => {
                      setScheduleType(t);
                      setScheduleValue(t === "interval" ? "30m" : "0 7 * * *");
                    }}
                    style={{
                      padding: "0.35rem 0.85rem", borderRadius: 6,
                      border: `1px solid ${scheduleType === t ? "#6c5ce7" : "#333"}`,
                      background: scheduleType === t ? "#6c5ce733" : "#1e1e28",
                      color: scheduleType === t ? "#9c8aff" : "#aaa",
                      cursor: "pointer", fontSize: 13,
                    }}
                  >
                    {t === "interval" ? "Interval" : "Cron"}
                  </button>
                ))}
              </div>
            </FieldRow>

            <FieldRow label={scheduleType === "interval" ? "Interval *" : "Cron expression *"}>
              {scheduleType === "interval" ? (
                <select value={scheduleValue} onChange={(e) => setScheduleValue(e.target.value)} style={inputStyle}>
                  {INTERVAL_OPTIONS.map((o) => (
                    <option key={o.value} value={o.value}>{o.label}</option>
                  ))}
                </select>
              ) : (
                <input value={scheduleValue} onChange={(e) => setScheduleValue(e.target.value)} style={inputStyle} placeholder="0 7 * * *" />
              )}
            </FieldRow>

            <FieldRow label="Task title *">
              <input value={taskTitle} onChange={(e) => setTaskTitle(e.target.value)} style={inputStyle} placeholder="What should the agent do?" />
            </FieldRow>

            <FieldRow label="Task description">
              <textarea
                value={taskDescription}
                onChange={(e) => setTaskDescription(e.target.value)}
                style={{ ...inputStyle, height: 64, resize: "vertical" }}
                placeholder="Optional additional context"
              />
            </FieldRow>

            {formError && (
              <div style={{ color: "#dc3545", fontSize: 12, marginBottom: "0.75rem" }}>{formError}</div>
            )}

            <div style={{ display: "flex", gap: "0.75rem", justifyContent: "flex-end" }}>
              <button
                onClick={() => setShowModal(false)}
                style={{ padding: "0.45rem 1rem", borderRadius: 6, border: "1px solid #444", background: "#2a2a38", color: "#aaa", cursor: "pointer" }}
              >
                Cancel
              </button>
              <button
                onClick={handleCreate}
                style={{ padding: "0.45rem 1rem", borderRadius: 6, border: "none", background: "#6c5ce7", color: "#fff", cursor: "pointer", fontWeight: 600 }}
              >
                Create
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function actionBtnStyle(color: string): React.CSSProperties {
  return {
    padding: "2px 8px",
    borderRadius: 5,
    border: `1px solid ${color}44`,
    background: `${color}22`,
    color,
    cursor: "pointer",
    fontSize: 11,
    fontWeight: 600,
  };
}
