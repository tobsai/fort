import { useEffect, useState, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useFortSocket } from "../contexts/FortSocketContext";
import type { AgentInfo, WSMessage } from "../types";

interface CreateAgentForm {
  name: string;
  description: string;
  emoji: string;
  soul: string;
  modelPreference: string;
}

const EMOJI_OPTIONS = ["🤖", "🧠", "🔍", "✍️", "📊", "🛠️", "🎯", "⚡", "🌐", "📝", "🔬", "💡"];

const DEFAULT_FORM: CreateAgentForm = {
  name: "",
  description: "",
  emoji: "🤖",
  soul: "",
  modelPreference: "",
};

export default function AgentsPage() {
  const { send, subscribe } = useFortSocket();
  const navigate = useNavigate();
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [selected, setSelected] = useState<AgentInfo | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState<CreateAgentForm>(DEFAULT_FORM);
  const [creating, setCreating] = useState(false);
  const [actionPending, setActionPending] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  const refreshAgents = useCallback(() => {
    send("agents.list");
  }, [send]);

  useEffect(() => {
    const unsub1 = subscribe("agents.response", (msg: WSMessage) => {
      setAgents((msg.payload as AgentInfo[]) || []);
    });
    const unsub2 = subscribe("agents.updated", (msg: WSMessage) => {
      setAgents((msg.payload as AgentInfo[]) || []);
      // Refresh selected agent if it's in the new list
      setSelected((prev) => {
        if (!prev) return null;
        const updated = (msg.payload as AgentInfo[]).find((a) => a.config.id === prev.config.id);
        return updated ?? null;
      });
    });
    send("agents");
    return () => { unsub1(); unsub2(); };
  }, [send, subscribe]);

  function openCreate() {
    setForm(DEFAULT_FORM);
    setShowCreate(true);
  }

  function closeCreate() {
    setShowCreate(false);
    setForm(DEFAULT_FORM);
  }

  async function handleCreate() {
    if (!form.name.trim()) return;
    setCreating(true);
    try {
      send("agent.create", {
        name: form.name.trim(),
        description: form.description.trim() || undefined,
        emoji: form.emoji || undefined,
        soul: form.soul.trim() || undefined,
        modelPreference: form.modelPreference || undefined,
      });
      // Listen for response once
      const unsub = subscribe("agent.create.response", () => {
        unsub();
        setCreating(false);
        closeCreate();
        refreshAgents();
      });
      const errUnsub = subscribe("error", () => {
        errUnsub();
        setCreating(false);
      });
    } catch {
      setCreating(false);
    }
  }

  async function handleStart(agentId: string, e: React.MouseEvent) {
    e.stopPropagation();
    setActionPending(agentId + ":start");
    send("agent.start", { id: agentId });
    const unsub = subscribe("agent.start.response", () => {
      unsub();
      setActionPending(null);
    });
  }

  async function handleStop(agentId: string, e: React.MouseEvent) {
    e.stopPropagation();
    setActionPending(agentId + ":stop");
    send("agent.stop", { id: agentId });
    const unsub = subscribe("agent.stop.response", () => {
      unsub();
      setActionPending(null);
    });
  }

  function handleDeleteClick(agentId: string, e: React.MouseEvent) {
    e.stopPropagation();
    setConfirmDelete(agentId);
  }

  function handleDeleteConfirm(agentId: string) {
    setActionPending(agentId + ":delete");
    setConfirmDelete(null);
    if (selected?.config.id === agentId) setSelected(null);
    send("agent.delete", { id: agentId });
    const unsub = subscribe("agent.delete.response", () => {
      unsub();
      setActionPending(null);
    });
  }

  function handleChatShortcut(agentId: string) {
    navigate(`/chat/${agentId}`);
  }

  return (
    <div className="agents-page">
      {/* Page header */}
      <div className="agents-header">
        <h2 className="agents-title">Agents</h2>
        <button className="btn-primary" onClick={openCreate}>
          + New Agent
        </button>
      </div>

      {/* Agent grid */}
      <div className="agents-grid">
        {agents.map((a) => {
          const cfg = a.config;
          const isPending = actionPending?.startsWith(cfg.id);
          return (
            <div
              key={cfg.id}
              className="agent-card"
              onClick={() => setSelected(a)}
            >
              <div className="agent-card-emoji">{(a as any).emoji || "🤖"}</div>
              <div className="agent-card-info">
                <div className="agent-card-name">{cfg.name}</div>
                <div className="agent-card-desc">{cfg.description}</div>
                <div className="agent-card-meta">
                  <span className="agent-task-count">{a.taskCount} tasks</span>
                </div>
              </div>
              <div className="agent-card-right">
                <span className={`agent-status-badge ${a.status}`}>
                  {a.status}
                </span>
                <div className="agent-actions" onClick={(e) => e.stopPropagation()}>
                  {a.status !== "running" ? (
                    <button
                      className="agent-action-btn start"
                      disabled={!!isPending}
                      onClick={(e) => handleStart(cfg.id, e)}
                      title="Start"
                    >
                      ▶
                    </button>
                  ) : (
                    <button
                      className="agent-action-btn stop"
                      disabled={!!isPending}
                      onClick={(e) => handleStop(cfg.id, e)}
                      title="Stop"
                    >
                      ■
                    </button>
                  )}
                  <button
                    className="agent-action-btn delete"
                    disabled={!!isPending}
                    onClick={(e) => handleDeleteClick(cfg.id, e)}
                    title="Delete"
                  >
                    ✕
                  </button>
                </div>
              </div>
            </div>
          );
        })}
        {agents.length === 0 && (
          <div className="empty-state">
            No agents yet.{" "}
            <button className="link-btn" onClick={openCreate}>
              Create your first agent
            </button>
          </div>
        )}
      </div>

      {/* Delete confirmation */}
      {confirmDelete && (
        <div className="modal-overlay" onClick={() => setConfirmDelete(null)}>
          <div className="modal modal-sm" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <span className="modal-title">Delete Agent</span>
              <button className="modal-close" onClick={() => setConfirmDelete(null)}>&times;</button>
            </div>
            <div className="modal-body">
              <p style={{ marginBottom: 20, color: "var(--text-secondary)" }}>
                This agent will be retired and removed from the list. Its memory and task history are preserved.
              </p>
              <div style={{ display: "flex", gap: 10, justifyContent: "flex-end" }}>
                <button className="btn-secondary" onClick={() => setConfirmDelete(null)}>Cancel</button>
                <button className="btn-danger" onClick={() => handleDeleteConfirm(confirmDelete)}>Delete</button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Agent detail modal */}
      {selected && !showCreate && !confirmDelete && (
        <div className="modal-overlay" onClick={(e) => {
          if (e.target === e.currentTarget) setSelected(null);
        }}>
          <div className="modal modal-lg" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <span className="modal-emoji">{(selected as any).emoji || "🤖"}</span>
              <span className="modal-title">{selected.config.name}</span>
              <button
                className="btn-secondary btn-sm"
                style={{ marginLeft: "auto", marginRight: 8 }}
                onClick={() => handleChatShortcut(selected.config.id)}
              >
                💬 Chat
              </button>
              <button className="modal-close" onClick={() => setSelected(null)}>&times;</button>
            </div>
            <div className="modal-body">
              <div className="agent-detail-grid">
                <div className="agent-detail-section">
                  <div className="detail-label">Status</div>
                  <span className={`agent-status-badge ${selected.status}`}>{selected.status}</span>
                </div>
                <div className="agent-detail-section">
                  <div className="detail-label">Tasks Completed</div>
                  <div className="detail-value">{selected.taskCount}</div>
                </div>
                {selected.config.capabilities.length > 0 && (
                  <div className="agent-detail-section full-width">
                    <div className="detail-label">Capabilities</div>
                    <div className="detail-chips">
                      {selected.config.capabilities.map((c) => (
                        <span key={c} className="chip">{c}</span>
                      ))}
                    </div>
                  </div>
                )}
                {selected.config.description && (
                  <div className="agent-detail-section full-width">
                    <div className="detail-label">Description</div>
                    <div className="detail-value">{selected.config.description}</div>
                  </div>
                )}
              </div>
              {(selected as any).soul && (
                <div style={{ marginTop: 16 }}>
                  <div className="detail-label" style={{ marginBottom: 8 }}>SOUL.md</div>
                  <div className="soul-content">
                    {(selected as any).soul}
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Create Agent modal */}
      {showCreate && (
        <div className="modal-overlay" onClick={(e) => {
          if (e.target === e.currentTarget) closeCreate();
        }}>
          <div className="modal modal-lg" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <span className="modal-title">New Agent</span>
              <button className="modal-close" onClick={closeCreate}>&times;</button>
            </div>
            <div className="modal-body">
              {/* Emoji picker */}
              <div className="form-group">
                <label className="form-label">Emoji</label>
                <div className="emoji-picker-row">
                  {EMOJI_OPTIONS.map((em) => (
                    <button
                      key={em}
                      className={`emoji-option${form.emoji === em ? " selected" : ""}`}
                      onClick={() => setForm((f) => ({ ...f, emoji: em }))}
                    >
                      {em}
                    </button>
                  ))}
                </div>
              </div>

              <div className="form-group">
                <label className="form-label">Name <span className="required">*</span></label>
                <input
                  className="form-input"
                  placeholder="e.g. Research Assistant"
                  value={form.name}
                  onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                  onKeyDown={(e) => { if (e.key === "Enter" && !creating) handleCreate(); }}
                  autoFocus
                />
              </div>

              <div className="form-group">
                <label className="form-label">Description</label>
                <textarea
                  className="form-input"
                  rows={2}
                  placeholder="What does this agent do?"
                  value={form.description}
                  onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
                />
              </div>

              <div className="form-group">
                <label className="form-label">Model Preference</label>
                <select
                  className="form-input model-select"
                  value={form.modelPreference}
                  onChange={(e) => setForm((f) => ({ ...f, modelPreference: e.target.value }))}
                >
                  <option value="">Default</option>
                  <option value="fast">Fast</option>
                  <option value="standard">Standard</option>
                  <option value="powerful">Powerful</option>
                </select>
              </div>

              <div className="form-group">
                <label className="form-label">Soul (SOUL.md)</label>
                <textarea
                  className="form-input soul-textarea"
                  rows={6}
                  placeholder="# Agent Name&#10;&#10;Describe the agent's personality, rules, and goals..."
                  value={form.soul}
                  onChange={(e) => setForm((f) => ({ ...f, soul: e.target.value }))}
                />
              </div>

              <div className="form-footer">
                <button className="btn-secondary" onClick={closeCreate} disabled={creating}>
                  Cancel
                </button>
                <button
                  className="btn-primary"
                  onClick={handleCreate}
                  disabled={creating || !form.name.trim()}
                >
                  {creating ? "Creating…" : "Create Agent"}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
